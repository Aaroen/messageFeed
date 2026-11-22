package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"messagefeed/internal/domain"

	"github.com/mmcdole/gofeed"
)

const weRSSAPI = "/api/v1/wx"
const weChatPublisherSourcePrefix = "https://weread.qq.com/web/bookDetail/"

var weChatPublisherID = regexp.MustCompile(`^MP_WXS_[0-9]{1,24}$`)
var weChatLoginOTP = regexp.MustCompile(`^[0-9]{4,8}$`)

type WeChatRSS struct {
	instances map[int64]*weRSSInstance
	client    *http.Client
}

type weRSSInstance struct {
	Endpoint string `json:"endpoint"`
	Username string `json:"username"`
	Password string `json:"password"`
	mutex    sync.Mutex
	token    string
	expires  time.Time
}

type WeChatRSSPublisher struct {
	ID    string `json:"book_id"`
	Title string `json:"title"`
}

type WeChatRSSArticle struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"-"`
}

type weRSSResponse struct {
	Code *int            `json:"code"`
	Data json.RawMessage `json:"data"`
}

func NewWeChatRSS(bindings string) (*WeChatRSS, error) {
	if strings.TrimSpace(bindings) == "" {
		return nil, nil
	}
	connection := &WeChatRSS{client: &http.Client{
		Timeout: 35 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
	if err := json.Unmarshal([]byte(bindings), &connection.instances); err != nil {
		return nil, errors.New("WECHAT_RSS_BINDINGS 的 JSON 格式无效")
	}
	endpoints := make(map[string]bool, len(connection.instances))
	for userID, instance := range connection.instances {
		if userID < 1 || instance == nil || instance.Username == "" || instance.Password == "" {
			return nil, errors.New("WeRSS 绑定缺少用户或管理凭据")
		}
		endpoint, err := url.Parse(instance.Endpoint)
		if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Hostname() == "" ||
			endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
			return nil, errors.New("WeRSS 实例地址无效")
		}
		instance.Endpoint = strings.TrimRight(instance.Endpoint, "/")
		if endpoints[strings.ToLower(endpoint.Hostname())] {
			return nil, errors.New("每个用户必须使用独立的 WeRSS 实例")
		}
		endpoints[strings.ToLower(endpoint.Hostname())] = true
	}
	return connection, nil
}

func (connection *WeChatRSS) Available(userID int64) bool {
	return connection != nil && connection.instances[userID] != nil
}

func (connection *WeChatRSS) Connected(ctx context.Context, userID int64) (bool, error) {
	var status struct {
		Configured bool `json:"configured"`
	}
	err := connection.call(ctx, userID, http.MethodGet, "/weread", nil, &status)
	return status.Configured, err
}

func (connection *WeChatRSS) StartLogin(ctx context.Context, userID int64) (string, error) {
	var qr struct {
		Code string `json:"code"`
	}
	if err := connection.call(ctx, userID, http.MethodGet, "/weread/qr/code", nil, &qr); err != nil {
		return "", err
	}
	if qr.Code != "/static/weread_qrcode.png" {
		return "", errors.New("WeRSS 返回了未知的二维码格式")
	}
	body, err := connection.request(ctx, connection.instances[userID], http.MethodGet, qr.Code, "", "", nil, 256<<10)
	if err != nil {
		return "", err
	}
	if http.DetectContentType(body) != "image/png" {
		return "", errors.New("WeRSS 二维码图片无效")
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(body), nil
}

type WeChatRSSLoginStatus struct {
	Connected bool   `json:"connected"`
	Challenge string `json:"challenge,omitempty"`
}

func (connection *WeChatRSS) LoginStatus(ctx context.Context, userID int64) (WeChatRSSLoginStatus, error) {
	var status struct {
		LoggedIn bool `json:"login_status"`
		Data     struct {
			LogicCode string `json:"logicCode"`
		} `json:"data"`
	}
	if err := connection.call(ctx, userID, http.MethodGet, "/weread/qr/status", nil, &status); err != nil {
		return WeChatRSSLoginStatus{}, err
	}
	result := WeChatRSSLoginStatus{Connected: status.LoggedIn}
	switch status.Data.LogicCode {
	case "NEED_OTP", "OTP_NOT_MATCH", "OTP_EXPIRED", "OTP_CHECKING", "COOKIE_INVALID":
		result.Challenge = status.Data.LogicCode
	}
	return result, nil
}

func (connection *WeChatRSS) SubmitLoginOTP(ctx context.Context, userID int64, otp string) error {
	if !weChatLoginOTP.MatchString(otp) {
		return fmt.Errorf("%w: 验证码须为 4 至 8 位数字", domain.ErrInvalidInput)
	}
	var response struct {
		Submitted bool `json:"submitted"`
	}
	err := connection.call(ctx, userID, http.MethodPost, "/weread/qr/otp", map[string]string{"otp": otp}, &response)
	if err != nil {
		return err
	}
	if !response.Submitted {
		return errors.New("微信验证码未提交")
	}
	return nil
}

func (connection *WeChatRSS) Publishers(ctx context.Context, userID int64) ([]WeChatRSSPublisher, error) {
	var shelf struct {
		Books []WeChatRSSPublisher `json:"books"`
	}
	if err := connection.call(ctx, userID, http.MethodPost, "/weread/bookshelf", nil, &shelf); err != nil {
		return nil, err
	}
	publishers := make([]WeChatRSSPublisher, 0, len(shelf.Books))
	for _, publisher := range shelf.Books {
		if weChatPublisherID.MatchString(publisher.ID) && publisher.Title != "" {
			publishers = append(publishers, publisher)
		}
	}
	return publishers, nil
}

func (connection *WeChatRSS) Articles(ctx context.Context, userID int64, publisherID string) ([]WeChatRSSArticle, error) {
	if !connection.Available(userID) {
		return nil, errors.New("当前用户未启用微信公众号连接")
	}
	if !weChatPublisherID.MatchString(publisherID) {
		return nil, errors.New("公众号编号无效")
	}
	body := struct {
		ID            string `json:"mp_id"`
		GatherContent bool   `json:"gather_content"`
	}{ID: publisherID, GatherContent: false}
	var collected json.RawMessage
	if err := connection.call(ctx, userID, http.MethodPost, "/weread/collect", body, &collected); err != nil {
		return nil, err
	}
	feedBody, err := connection.request(ctx, connection.instances[userID], http.MethodGet, "/feed/"+publisherID+".xml?limit=20", "", "", nil, 4<<20)
	if err != nil {
		return nil, err
	}
	feed, err := gofeed.NewParser().Parse(bytes.NewReader(feedBody))
	if err != nil {
		return nil, errors.New("WeRSS 返回的公众号订阅无效")
	}
	articles := make([]WeChatRSSArticle, 0, len(feed.Items))
	for _, item := range feed.Items {
		original, err := url.Parse(item.Link)
		if err == nil && original.Scheme == "https" && original.User == nil &&
			(original.Host == "mp.weixin.qq.com" || original.Host == "weread.qq.com") && item.Title != "" {
			articles = append(articles, WeChatRSSArticle{Title: item.Title, URL: item.Link, Summary: item.Description})
		}
	}
	return articles, nil
}

func (s *SourceService) SubscribeWeChatPublisher(ctx context.Context, provider *WeChatRSS, userID int64, publisherID string) (FetchSourceResult, error) {
	if !weChatPublisherID.MatchString(publisherID) {
		return FetchSourceResult{}, fmt.Errorf("%w: 公众号编号无效", domain.ErrInvalidInput)
	}
	publishers, err := provider.Publishers(ctx, userID)
	if err != nil {
		return FetchSourceResult{}, err
	}
	for _, publisher := range publishers {
		if publisher.ID != publisherID {
			continue
		}
		source, err := s.createOrReactivateSource(ctx, CreateSourceInput{
			UserID: userID, Name: publisher.Title, Type: domain.SourceTypeInternal, URL: weChatPublisherSourcePrefix + publisher.ID,
		})
		if err != nil {
			return FetchSourceResult{}, err
		}
		return s.TriggerFetch(ctx, FetchSourceInput{UserID: userID, ID: source.ID})
	}
	return FetchSourceResult{}, fmt.Errorf("%w: 当前微信账号未返回该公众号", domain.ErrNotFound)
}

type weChatSourceFetcher struct {
	provider *WeChatRSS
	public   FeedFetcher
}

func WithWeChatSourceFetcher(public FeedFetcher, provider *WeChatRSS) FeedFetcher {
	return weChatSourceFetcher{provider: provider, public: public}
}

func (fetcher weChatSourceFetcher) Fetch(ctx context.Context, source domain.Source) (domain.FeedFetchResult, error) {
	publisherID, matched := strings.CutPrefix(source.NormalizedURL, weChatPublisherSourcePrefix)
	if source.Type != domain.SourceTypeInternal || !matched {
		return fetcher.public.Fetch(ctx, source)
	}
	articles, err := fetcher.provider.Articles(ctx, source.UserID, publisherID)
	if err != nil {
		return domain.FeedFetchResult{}, err
	}
	fetchedAt := time.Now().UTC()
	items := make([]domain.Item, 0, len(articles))
	for _, article := range articles {
		normalizedURL, _, err := NormalizeSourceURL(article.URL)
		if err != nil {
			return domain.FeedFetchResult{}, err
		}
		hash := sha256.Sum256([]byte(article.Title + "\n" + normalizedURL + "\n" + article.Summary))
		items = append(items, domain.Item{
			SourceID: source.ID, Title: article.Title, URL: article.URL, NormalizedURL: normalizedURL,
			Author: source.Name, ContentHash: fmt.Sprintf("%x", hash), FetchedAt: fetchedAt,
		})
	}
	return domain.FeedFetchResult{Items: items}, nil
}

func (connection *WeChatRSS) call(ctx context.Context, userID int64, method string, path string, input any, output any) error {
	if !connection.Available(userID) {
		return errors.New("当前用户未启用微信公众号连接")
	}
	instance := connection.instances[userID]
	token, err := connection.token(ctx, instance)
	if err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	return connection.readJSON(ctx, instance, method, weRSSAPI+path, token, "application/json", body, output)
}

func (connection *WeChatRSS) token(ctx context.Context, instance *weRSSInstance) (string, error) {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.token != "" && time.Now().Before(instance.expires) {
		return instance.token, nil
	}
	form := url.Values{"username": {instance.Username}, "password": {instance.Password}}
	var session struct {
		Token string `json:"access_token"`
	}
	err := connection.readJSON(ctx, instance, http.MethodPost, weRSSAPI+"/auth/login", "", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), &session)
	if err != nil || session.Token == "" {
		return "", errors.New("无法连接当前用户的 WeRSS 服务")
	}
	instance.token = session.Token
	instance.expires = time.Now().Add(10 * time.Minute)
	return session.Token, nil
}

func (connection *WeChatRSS) readJSON(ctx context.Context, instance *weRSSInstance, method string, path string, token string, contentType string, body io.Reader, output any) error {
	response, err := connection.request(ctx, instance, method, path, token, contentType, body, 4<<20)
	if err != nil {
		return err
	}
	var reply weRSSResponse
	if err := json.Unmarshal(response, &reply); err != nil || reply.Code == nil {
		return errors.New("WeRSS 响应格式无效")
	}
	if *reply.Code != 0 {
		return fmt.Errorf("WeRSS 当前无法处理请求（状态 %d）", *reply.Code)
	}
	if err := json.Unmarshal(reply.Data, output); err != nil {
		return errors.New("WeRSS 返回数据格式无效")
	}
	return nil
}

func (connection *WeChatRSS) request(ctx context.Context, instance *weRSSInstance, method string, path string, token string, contentType string, body io.Reader, size int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, instance.Endpoint+path, body)
	if err != nil {
		return nil, errors.New("WeRSS 请求格式无效")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := connection.client.Do(request)
	if err != nil {
		return nil, errors.New("WeRSS 暂时无法连接")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("WeRSS 请求失败（HTTP %d）", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, size+1))
	if err != nil || int64(len(data)) > size {
		return nil, errors.New("WeRSS 响应无法读取")
	}
	return data, nil
}
