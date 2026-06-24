package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultWeChatWorkOAuthAPIBaseURL = "https://qyapi.weixin.qq.com"
	wechatWorkOAuthGetTokenOp        = "wechat_work_oauth_gettoken"
	wechatWorkOAuthGetUserInfoOp     = "wechat_work_oauth_getuserinfo"
)

type WeChatWorkOAuthConfig struct {
	CorpID     string
	Secret     string
	APIBaseURL string
	HTTPClient *http.Client
	Now        func() time.Time
}

type WeChatWorkOAuthClient struct {
	corpID     string
	secret     string
	apiBaseURL string
	httpClient *http.Client
	now        func() time.Time

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

type WeChatWorkOAuthUser struct {
	UserID   string
	OpenID   string
	DeviceID string
	Name     string
}

type weChatWorkOAuthTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type weChatWorkOAuthUserInfoResponse struct {
	ErrCode       int    `json:"errcode"`
	ErrMsg        string `json:"errmsg"`
	UserID        string `json:"UserId"`
	UserIDUpper   string `json:"UserID"`
	UserIDLower   string `json:"userid"`
	UserIDSnake   string `json:"user_id"`
	OpenID        string `json:"OpenId"`
	OpenIDUpper   string `json:"OpenID"`
	OpenIDLower   string `json:"openid"`
	DeviceID      string `json:"DeviceId"`
	DeviceIDUpper string `json:"DeviceID"`
	DeviceIDLower string `json:"deviceid"`
}

func NewWeChatWorkOAuthClient(config WeChatWorkOAuthConfig) (*WeChatWorkOAuthClient, error) {
	apiBaseURL := strings.TrimRight(strings.TrimSpace(config.APIBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultWeChatWorkOAuthAPIBaseURL
	}
	if parsed, err := url.Parse(apiBaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if err == nil {
			err = fmt.Errorf("scheme and host are required")
		}
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_oauth_invalid_base_url", "invalid wechat work oauth api base url", "service.wechat_work_oauth.new", false, err)
	}
	corpID := strings.TrimSpace(config.CorpID)
	secret := strings.TrimSpace(config.Secret)
	if corpID == "" || secret == "" {
		return nil, domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_oauth_missing_config", "wechat work oauth config is incomplete", "service.wechat_work_oauth.new", false, nil)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:   10 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &WeChatWorkOAuthClient{
		corpID:     corpID,
		secret:     secret,
		apiBaseURL: apiBaseURL,
		httpClient: httpClient,
		now:        now,
	}, nil
}

func (c *WeChatWorkOAuthClient) ExchangeCode(ctx context.Context, code string) (WeChatWorkOAuthUser, error) {
	ctx, span := observability.StartSpan(ctx, "service.wechat_work_oauth.exchange_code")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	code = strings.TrimSpace(code)
	if c == nil || c.httpClient == nil {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_oauth_unavailable", "wechat work oauth client is unavailable", "service.wechat_work_oauth.exchange_code", true, nil)
		return WeChatWorkOAuthUser{}, opErr
	}
	if code == "" {
		opErr = fmt.Errorf("%w: code is required", domain.ErrInvalidInput)
		return WeChatWorkOAuthUser{}, opErr
	}
	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		opErr = err
		return WeChatWorkOAuthUser{}, err
	}

	endpoint := c.apiBaseURL + "/cgi-bin/auth/getuserinfo?access_token=" + url.QueryEscape(accessToken) + "&code=" + url.QueryEscape(code)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		opErr = err
		return WeChatWorkOAuthUser{}, err
	}
	startedAt := time.Now()
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		recordWeChatWorkOAuthExternalHTTPRequest(wechatWorkOAuthGetUserInfoOp, c.apiBaseURL, "error", time.Since(startedAt))
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_oauth_userinfo_failed", "wechat work oauth userinfo request failed", "service.wechat_work_oauth.exchange_code", true, err)
		return WeChatWorkOAuthUser{}, opErr
	}
	defer httpResponse.Body.Close()
	recordWeChatWorkOAuthExternalHTTPRequest(wechatWorkOAuthGetUserInfoOp, c.apiBaseURL, strconv.Itoa(httpResponse.StatusCode), time.Since(startedAt))
	span.SetAttributes(attribute.Int("http.response.status_code", httpResponse.StatusCode))

	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		opErr = err
		return WeChatWorkOAuthUser{}, err
	}
	var decoded weChatWorkOAuthUserInfoResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_oauth_invalid_response", "wechat work oauth userinfo response is invalid", "service.wechat_work_oauth.exchange_code", true, err)
		return WeChatWorkOAuthUser{}, opErr
	}
	span.SetAttributes(attribute.Int("wechat_work.errcode", decoded.ErrCode))
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices || decoded.ErrCode != 0 {
		message := strings.TrimSpace(decoded.ErrMsg)
		if message == "" {
			message = http.StatusText(httpResponse.StatusCode)
		}
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_oauth_provider_error", message, "service.wechat_work_oauth.exchange_code", true, nil)
		return WeChatWorkOAuthUser{}, opErr
	}
	return WeChatWorkOAuthUser{
		UserID:   firstNonEmpty(decoded.UserID, decoded.UserIDUpper, decoded.UserIDLower, decoded.UserIDSnake),
		OpenID:   firstNonEmpty(decoded.OpenID, decoded.OpenIDUpper, decoded.OpenIDLower),
		DeviceID: firstNonEmpty(decoded.DeviceID, decoded.DeviceIDUpper, decoded.DeviceIDLower),
	}, nil
}

func (c *WeChatWorkOAuthClient) getAccessToken(ctx context.Context) (string, error) {
	ctx, span := observability.StartSpan(ctx, "service.wechat_work_oauth.get_token")
	cacheHit := false
	var opErr error
	defer func() {
		span.SetAttributes(attribute.Bool("wechat_work.token_cache_hit", cacheHit))
		observability.EndSpan(span, opErr)
	}()

	c.mu.Lock()
	if c.accessToken != "" && c.now().Before(c.expiresAt) {
		token := c.accessToken
		c.mu.Unlock()
		cacheHit = true
		return token, nil
	}
	c.mu.Unlock()

	endpoint := c.apiBaseURL + "/cgi-bin/gettoken?corpid=" + url.QueryEscape(c.corpID) + "&corpsecret=" + url.QueryEscape(c.secret)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		opErr = err
		return "", err
	}
	startedAt := time.Now()
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		recordWeChatWorkOAuthExternalHTTPRequest(wechatWorkOAuthGetTokenOp, c.apiBaseURL, "error", time.Since(startedAt))
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_oauth_token_failed", "wechat work oauth token request failed", "service.wechat_work_oauth.token", true, err)
		return "", opErr
	}
	defer httpResponse.Body.Close()
	recordWeChatWorkOAuthExternalHTTPRequest(wechatWorkOAuthGetTokenOp, c.apiBaseURL, strconv.Itoa(httpResponse.StatusCode), time.Since(startedAt))

	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		opErr = err
		return "", err
	}
	var decoded weChatWorkOAuthTokenResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_oauth_token_invalid_response", "wechat work oauth token response is invalid", "service.wechat_work_oauth.token", true, err)
		return "", opErr
	}
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices || decoded.ErrCode != 0 || decoded.AccessToken == "" {
		message := strings.TrimSpace(decoded.ErrMsg)
		if message == "" {
			message = http.StatusText(httpResponse.StatusCode)
		}
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_oauth_token_provider_error", message, "service.wechat_work_oauth.token", true, nil)
		return "", opErr
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

func recordWeChatWorkOAuthExternalHTTPRequest(operation string, apiBaseURL string, status string, duration time.Duration) {
	if status == "" {
		status = "unknown"
	}
	host := "unknown"
	if parsed, err := url.Parse(apiBaseURL); err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	metrics.ExternalHTTPRequestsTotal.WithLabelValues(operation, host, status).Inc()
	metrics.ExternalHTTPRequestDuration.WithLabelValues(operation, host).Observe(duration.Seconds())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
