package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"messagefeed/internal/domain"
)

const (
	defaultWeChatWorkAPIBaseURL = "https://qyapi.weixin.qq.com"
	weChatWorkTextByteLimit     = 2048
)

type WeChatWorkAppConfig struct {
	CorpID     string
	AgentID    string
	Secret     string
	APIBaseURL string
	HTTPClient *http.Client
	Now        func() time.Time
}

type WeChatWorkAppClient struct {
	corpID     string
	agentID    string
	secret     string
	apiBaseURL string
	httpClient *http.Client
	now        func() time.Time

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

type WeChatWorkTextMessage struct {
	ToUser  string
	ToParty string
	ToTag   string
	Content string
}

type WeChatWorkSendResult struct {
	ErrCode        int
	ErrMsg         string
	InvalidUser    string
	InvalidParty   string
	InvalidTag     string
	UnlicensedUser string
	MessageID      string
	ResponseBody   string
}

type tokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type sendTextRequest struct {
	ToUser  string `json:"touser,omitempty"`
	ToParty string `json:"toparty,omitempty"`
	ToTag   string `json:"totag,omitempty"`
	MsgType string `json:"msgtype"`
	AgentID string `json:"agentid"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

type sendTextResponse struct {
	ErrCode        int    `json:"errcode"`
	ErrMsg         string `json:"errmsg"`
	InvalidUser    string `json:"invaliduser"`
	InvalidParty   string `json:"invalidparty"`
	InvalidTag     string `json:"invalidtag"`
	UnlicensedUser string `json:"unlicenseduser"`
	MessageID      string `json:"msgid"`
}

func NewWeChatWorkAppClient(config WeChatWorkAppConfig) (*WeChatWorkAppClient, error) {
	apiBaseURL := strings.TrimRight(strings.TrimSpace(config.APIBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultWeChatWorkAPIBaseURL
	}
	if parsed, err := url.Parse(apiBaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if err == nil {
			err = fmt.Errorf("scheme and host are required")
		}
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_invalid_base_url", "invalid wechat work api base url", "notifier.wechat_work.new", false, err)
	}
	corpID := strings.TrimSpace(config.CorpID)
	agentID := strings.TrimSpace(config.AgentID)
	secret := strings.TrimSpace(config.Secret)
	if corpID == "" || agentID == "" || secret == "" {
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_missing_config", "wechat work app config is incomplete", "notifier.wechat_work.new", false, nil)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &WeChatWorkAppClient{
		corpID:     corpID,
		agentID:    agentID,
		secret:     secret,
		apiBaseURL: apiBaseURL,
		httpClient: httpClient,
		now:        now,
	}, nil
}

func (c *WeChatWorkAppClient) SendText(ctx context.Context, message WeChatWorkTextMessage) (WeChatWorkSendResult, error) {
	if c == nil || c.httpClient == nil {
		return WeChatWorkSendResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_unavailable", "wechat work app client is unavailable", "notifier.wechat_work.send_text", true, nil)
	}
	message.ToUser = strings.TrimSpace(message.ToUser)
	message.ToParty = strings.TrimSpace(message.ToParty)
	message.ToTag = strings.TrimSpace(message.ToTag)
	message.Content = truncateUTF8Bytes(strings.TrimSpace(message.Content), weChatWorkTextByteLimit)
	if message.ToUser == "" && message.ToParty == "" && message.ToTag == "" {
		return WeChatWorkSendResult{}, domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_missing_recipient", "wechat work recipient is required", "notifier.wechat_work.send_text", false, nil)
	}
	if message.Content == "" {
		return WeChatWorkSendResult{}, domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_empty_content", "wechat work content is required", "notifier.wechat_work.send_text", false, nil)
	}

	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		return WeChatWorkSendResult{}, err
	}

	requestBody := sendTextRequest{
		ToUser:  message.ToUser,
		ToParty: message.ToParty,
		ToTag:   message.ToTag,
		MsgType: "text",
		AgentID: c.agentID,
	}
	requestBody.Text.Content = message.Content
	body, err := json.Marshal(requestBody)
	if err != nil {
		return WeChatWorkSendResult{}, err
	}

	endpoint := c.apiBaseURL + "/cgi-bin/message/send?access_token=" + url.QueryEscape(accessToken)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return WeChatWorkSendResult{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return WeChatWorkSendResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_send_failed", "wechat work send request failed", "notifier.wechat_work.send_text", true, err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		return WeChatWorkSendResult{}, err
	}
	var decoded sendTextResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return WeChatWorkSendResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_invalid_response", "wechat work send response is invalid", "notifier.wechat_work.send_text", true, err)
	}
	result := WeChatWorkSendResult{
		ErrCode:        decoded.ErrCode,
		ErrMsg:         decoded.ErrMsg,
		InvalidUser:    decoded.InvalidUser,
		InvalidParty:   decoded.InvalidParty,
		InvalidTag:     decoded.InvalidTag,
		UnlicensedUser: decoded.UnlicensedUser,
		MessageID:      decoded.MessageID,
		ResponseBody:   string(responseBody),
	}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices || decoded.ErrCode != 0 {
		message := strings.TrimSpace(decoded.ErrMsg)
		if message == "" {
			message = http.StatusText(httpResponse.StatusCode)
		}
		return result, domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_provider_error", message, "notifier.wechat_work.send_text", true, nil)
	}
	return result, nil
}

func (c *WeChatWorkAppClient) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.accessToken != "" && c.now().Before(c.expiresAt) {
		token := c.accessToken
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	endpoint := c.apiBaseURL + "/cgi-bin/gettoken?corpid=" + url.QueryEscape(c.corpID) + "&corpsecret=" + url.QueryEscape(c.secret)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return "", domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_token_failed", "wechat work token request failed", "notifier.wechat_work.token", true, err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var decoded tokenResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_token_invalid_response", "wechat work token response is invalid", "notifier.wechat_work.token", true, err)
	}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices || decoded.ErrCode != 0 || decoded.AccessToken == "" {
		message := strings.TrimSpace(decoded.ErrMsg)
		if message == "" {
			message = http.StatusText(httpResponse.StatusCode)
		}
		return "", domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_token_provider_error", message, "notifier.wechat_work.token", true, nil)
	}
	expiresIn := time.Duration(decoded.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 2 * time.Hour
	}
	expiresAt := c.now().Add(expiresIn - 5*time.Minute)
	if expiresAt.Before(c.now()) {
		expiresAt = c.now().Add(expiresIn / 2)
	}

	c.mu.Lock()
	c.accessToken = decoded.AccessToken
	c.expiresAt = expiresAt
	c.mu.Unlock()
	return decoded.AccessToken, nil
}

func truncateUTF8Bytes(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	output := make([]rune, 0, len(value))
	total := 0
	for _, r := range value {
		size := len(string(r))
		if total+size > limit {
			break
		}
		output = append(output, r)
		total += size
	}
	return string(output)
}
