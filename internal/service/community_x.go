package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"messagefeed/internal/domain"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html"
)

const xSourcePrefix = "https://x.com/"

var xAccountName = regexp.MustCompile(`^[a-z0-9_]{1,15}$`)
var xPostPath = regexp.MustCompile(`^/(?:[A-Za-z0-9_]{1,15}/status|i/web/status|i/status)/([0-9]{1,24})/?$`)
var ErrXAuthorization = errors.New("请在订阅管理中授权 X 账号")
var ErrXInputFocus = errors.New("请先选中 X 登录窗口中的输入框")

type XConnector struct {
	BrowserEndpoints map[int64]*url.URL
	client           *http.Client
}

func NewXConnector(bindings string) (*XConnector, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	connection := &XConnector{BrowserEndpoints: make(map[int64]*url.URL), client: &http.Client{
		Transport: transport, Timeout: 40 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	if strings.TrimSpace(bindings) == "" {
		return connection, nil
	}
	var endpoints map[int64]string
	if err := json.Unmarshal([]byte(bindings), &endpoints); err != nil {
		return nil, errors.New("X_BROWSER_BINDINGS 的 JSON 格式无效")
	}
	hosts := make(map[string]bool, len(endpoints))
	for userID, address := range endpoints {
		endpoint, err := url.Parse(address)
		if err != nil || userID < 1 || endpoint.Scheme != "http" || endpoint.Hostname() == "" ||
			endpoint.User != nil || endpoint.Path != "" || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" {
			return nil, fmt.Errorf("用户 %d 的 X 浏览器地址无效", userID)
		}
		host := strings.ToLower(strings.TrimSuffix(endpoint.Hostname(), "."))
		if hosts[host] {
			return nil, errors.New("每个用户必须使用独立的 X 浏览器实例")
		}
		hosts[host] = true
		connection.BrowserEndpoints[userID] = endpoint
	}
	return connection, nil
}

func (connection *XConnector) Connected(ctx context.Context, userID int64) (bool, error) {
	body, err := connection.request(ctx, userID, http.MethodGet, "/status", nil)
	if err != nil {
		return false, err
	}
	var status struct {
		Connected bool `json:"connected"`
	}
	err = json.Unmarshal(body, &status)
	return status.Connected, err
}

func (connection *XConnector) InsertText(ctx context.Context, userID int64, text string) error {
	if text == "" || utf8.RuneCountInString(text) > 1024 || strings.ContainsAny(text, "\r\n\x00") {
		return fmt.Errorf("%w: 请输入不超过 1024 个字符的单行文本", domain.ErrInvalidInput)
	}
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	_, err = connection.request(ctx, userID, http.MethodPost, "/input", bytes.NewReader(body))
	return err
}

func (connection *XConnector) request(ctx context.Context, userID int64, method string, path string, body io.Reader) ([]byte, error) {
	endpoint := connection.BrowserEndpoints[userID]
	if endpoint == nil {
		return nil, ErrXAuthorization
	}
	address := url.URL{Scheme: "http", Host: net.JoinHostPort(endpoint.Hostname(), "1201"), Path: path}
	request, err := http.NewRequestWithContext(ctx, method, address.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := connection.client.Do(request)
	if err != nil {
		return nil, errors.New("暂时无法连接 X 服务")
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
	case http.StatusUnauthorized:
		return nil, ErrXAuthorization
	case http.StatusConflict:
		return nil, ErrXInputFocus
	case http.StatusNotFound:
		return nil, fmt.Errorf("%w: X 未返回此账号的时间线", domain.ErrNotFound)
	case http.StatusTooManyRequests:
		return nil, errors.New("X 暂时限制刷新频率，请稍后再试")
	default:
		return nil, errors.New("暂时无法从 X 读取数据，请稍后重试")
	}
	const bodyLimit = 4 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, bodyLimit+1))
	if err != nil || len(data) > bodyLimit {
		return nil, errors.New("X 服务返回的数据无法读取")
	}
	return data, nil
}

func xSubscription(value string) (sourceURL string, route string, name string, err error) {
	account := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "@"))
	if account == "" && strings.TrimSpace(value) == "" {
		return xSourcePrefix + "home", "/twitter/home_latest", "X 关注动态", nil
	}
	if !xAccountName.MatchString(account) || account == "home" {
		return "", "", "", fmt.Errorf("%w: 请输入有效的 X 用户名", domain.ErrInvalidInput)
	}
	return xSourcePrefix + account, "/twitter/user/" + account, "X @" + account, nil
}

func (s *SourceService) SubscribeX(ctx context.Context, connection *XConnector, userID int64, account string) (FetchSourceResult, error) {
	address, _, name, err := xSubscription(account)
	if err != nil {
		return FetchSourceResult{}, err
	}
	connected, err := connection.Connected(ctx, userID)
	if err != nil {
		return FetchSourceResult{}, err
	}
	if !connected {
		return FetchSourceResult{}, ErrXAuthorization
	}
	next := s.now().UTC().Add(time.Hour)
	source, err := s.createOrReactivateSource(ctx, CreateSourceInput{
		UserID: userID, Name: name, Type: domain.SourceTypeInternal, URL: address, NextFetchAt: &next,
	})
	if err != nil {
		return FetchSourceResult{}, err
	}
	if source.Type != domain.SourceTypeInternal {
		return FetchSourceResult{}, fmt.Errorf("%w: 此 X 地址已被其他订阅使用", domain.ErrConflict)
	}
	return s.TriggerFetch(ctx, FetchSourceInput{UserID: userID, ID: source.ID})
}

type xSourceFetcher struct {
	connection *XConnector
	public     FeedFetcher
}

func WithXSourceFetcher(public FeedFetcher, connection *XConnector) FeedFetcher {
	return xSourceFetcher{connection: connection, public: public}
}

func (fetcher xSourceFetcher) Fetch(ctx context.Context, source domain.Source) (domain.FeedFetchResult, error) {
	account, matched := strings.CutPrefix(source.NormalizedURL, xSourcePrefix)
	if source.Type != domain.SourceTypeInternal || !matched {
		return fetcher.public.Fetch(ctx, source)
	}
	if account == "home" {
		account = ""
	}
	_, route, _, err := xSubscription(account)
	if err != nil {
		return domain.FeedFetchResult{}, err
	}
	body, err := fetcher.connection.request(ctx, source.UserID, http.MethodGet, route, nil)
	if err != nil {
		return domain.FeedFetchResult{}, err
	}
	return parseXFeed(body, source.ID, time.Now().UTC())
}

func parseXFeed(body []byte, sourceID int64, fetchedAt time.Time) (domain.FeedFetchResult, error) {
	feed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return domain.FeedFetchResult{}, errors.New("X 返回的订阅数据无效")
	}
	items := make([]domain.Item, 0, len(feed.Items))
	seen := make(map[string]bool, len(feed.Items))
	for _, post := range feed.Items {
		link, err := url.Parse(post.Link)
		if err != nil || link.Scheme != "https" || link.User != nil ||
			(link.Host != "x.com" && link.Host != "twitter.com" && link.Host != "www.x.com" && link.Host != "www.twitter.com") {
			continue
		}
		match := xPostPath.FindStringSubmatch(link.Path)
		if len(match) != 2 || seen[match[1]] {
			continue
		}
		postID := match[1]
		seen[postID] = true
		link.Host, link.RawQuery, link.Fragment, link.RawPath, link.ForceQuery = "x.com", "", "", "", false
		content := xPlainText(post.Description)
		title := xPlainText(post.Title)
		if title == "" {
			title = "X 推文"
		}
		normalized := xSourcePrefix + "i/web/status/" + postID
		item := domain.Item{SourceID: sourceID, Title: title, URL: link.String(), NormalizedURL: normalized, RawGUID: postID,
			ContentHash: fmt.Sprintf("%x", sha256.Sum256([]byte(title+"\n"+content))), ContentSnippet: content, FetchedAt: fetchedAt}
		if len(post.Authors) > 0 && post.Authors[0] != nil {
			item.Author = post.Authors[0].Name
		}
		if post.PublishedParsed != nil && !post.PublishedParsed.After(fetchedAt) {
			published := post.PublishedParsed.UTC()
			item.PublishedAt, item.PublicationVerified = &published, true
		}
		items = append(items, item)
	}
	if len(feed.Items) > 0 && len(items) == 0 {
		return domain.FeedFetchResult{}, errors.New("X 返回的数据没有可识别的推文地址")
	}
	return domain.FeedFetchResult{Items: items}, nil
}

func xPlainText(value string) string {
	tokens := html.NewTokenizer(strings.NewReader(value))
	var text strings.Builder
	for tokenType := tokens.Next(); tokenType != html.ErrorToken; tokenType = tokens.Next() {
		if tokenType == html.TextToken {
			text.Write(tokens.Text())
		} else if tokenType == html.StartTagToken || tokenType == html.EndTagToken || tokenType == html.SelfClosingTagToken {
			tag, _ := tokens.TagName()
			switch string(tag) {
			case "p", "br", "div", "blockquote", "li":
				text.WriteByte(' ')
			}
		}
	}
	return strings.Join(strings.Fields(text.String()), " ")
}
