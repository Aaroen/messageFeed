package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

const (
	defaultWeChatWorkAPIBaseURL = "https://qyapi.weixin.qq.com"
	WeChatWorkTextByteLimit     = 2048
	weChatWorkAppChannel        = "wechat_work_app"
	weChatWorkMessageSendOp     = "wechat_work_message_send"
	weChatWorkGetTokenOp        = "wechat_work_gettoken"
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

type WeChatWorkTemplateCardMessage struct {
	ToUser       string
	ToParty      string
	ToTag        string
	Title        string
	Description  string
	URL          string
	FallbackText string
	Buttons      []WeChatWorkTemplateCardButton
}

type WeChatWorkTemplateCardButton struct {
	Key   string
	Text  string
	URL   string
	Style string
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

type sendTemplateCardRequest struct {
	ToUser       string `json:"touser,omitempty"`
	ToParty      string `json:"toparty,omitempty"`
	ToTag        string `json:"totag,omitempty"`
	MsgType      string `json:"msgtype"`
	AgentID      string `json:"agentid"`
	TemplateCard struct {
		CardType  string `json:"card_type"`
		MainTitle struct {
			Title string `json:"title"`
			Desc  string `json:"desc,omitempty"`
		} `json:"main_title"`
		JumpList []struct {
			Type  int    `json:"type"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"jump_list,omitempty"`
		CardAction struct {
			Type int    `json:"type"`
			URL  string `json:"url"`
		} `json:"card_action"`
	} `json:"template_card"`
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
		httpClient = &http.Client{
			Timeout:   10 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
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
	startedAt := time.Now()
	agentID := ""
	if c != nil {
		agentID = c.agentID
	}
	ctx, span := observability.StartSpan(ctx, "notifier.wechat_work.send_text",
		attribute.String("notification.channel", weChatWorkAppChannel),
		attribute.String("wechat_work.agent_id", agentID),
	)
	status := "failed"
	var sendErr error
	defer func() {
		span.SetAttributes(attribute.String("notification.status", status))
		metrics.NotificationsTotal.WithLabelValues(weChatWorkAppChannel, status).Inc()
		metrics.NotificationDuration.WithLabelValues(weChatWorkAppChannel, status).Observe(time.Since(startedAt).Seconds())
		observability.EndSpan(span, sendErr)
	}()

	if c == nil || c.httpClient == nil {
		sendErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_unavailable", "wechat work app client is unavailable", "notifier.wechat_work.send_text", true, nil)
		return WeChatWorkSendResult{}, sendErr
	}
	message.ToUser = strings.TrimSpace(message.ToUser)
	message.ToParty = strings.TrimSpace(message.ToParty)
	message.ToTag = strings.TrimSpace(message.ToTag)
	message.Content = truncateUTF8Bytes(strings.TrimSpace(message.Content), WeChatWorkTextByteLimit)
	span.SetAttributes(
		attribute.Bool("wechat_work.has_touser", message.ToUser != ""),
		attribute.Bool("wechat_work.has_toparty", message.ToParty != ""),
		attribute.Bool("wechat_work.has_totag", message.ToTag != ""),
		attribute.Int("notification.content_bytes", len([]byte(message.Content))),
	)
	if message.ToUser == "" && message.ToParty == "" && message.ToTag == "" {
		sendErr = domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_missing_recipient", "wechat work recipient is required", "notifier.wechat_work.send_text", false, nil)
		return WeChatWorkSendResult{}, sendErr
	}
	if message.Content == "" {
		sendErr = domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_empty_content", "wechat work content is required", "notifier.wechat_work.send_text", false, nil)
		return WeChatWorkSendResult{}, sendErr
	}

	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		sendErr = err
		return WeChatWorkSendResult{}, sendErr
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
		sendErr = err
		return WeChatWorkSendResult{}, sendErr
	}

	endpoint := c.apiBaseURL + "/cgi-bin/message/send?access_token=" + url.QueryEscape(accessToken)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		sendErr = err
		return WeChatWorkSendResult{}, sendErr
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpStartedAt := time.Now()
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		recordWeChatWorkExternalHTTPRequest(weChatWorkMessageSendOp, c.apiBaseURL, "error", time.Since(httpStartedAt))
		sendErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_send_failed", "wechat work send request failed", "notifier.wechat_work.send_text", true, err)
		return WeChatWorkSendResult{}, sendErr
	}
	defer httpResponse.Body.Close()
	httpStatus := strconv.Itoa(httpResponse.StatusCode)
	recordWeChatWorkExternalHTTPRequest(weChatWorkMessageSendOp, c.apiBaseURL, httpStatus, time.Since(httpStartedAt))
	span.SetAttributes(attribute.Int("http.response.status_code", httpResponse.StatusCode))

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		sendErr = err
		return WeChatWorkSendResult{}, sendErr
	}
	var decoded sendTextResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		sendErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_invalid_response", "wechat work send response is invalid", "notifier.wechat_work.send_text", true, err)
		return WeChatWorkSendResult{}, sendErr
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
	span.SetAttributes(attribute.Int("wechat_work.errcode", decoded.ErrCode))
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices || decoded.ErrCode != 0 {
		message := strings.TrimSpace(decoded.ErrMsg)
		if message == "" {
			message = http.StatusText(httpResponse.StatusCode)
		}
		sendErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_provider_error", message, "notifier.wechat_work.send_text", true, nil)
		return result, sendErr
	}
	status = "success"
	return result, nil
}

func (c *WeChatWorkAppClient) SendTemplateCard(ctx context.Context, message WeChatWorkTemplateCardMessage) (WeChatWorkSendResult, error) {
	startedAt := time.Now()
	agentID := ""
	if c != nil {
		agentID = c.agentID
	}
	ctx, span := observability.StartSpan(ctx, "notifier.wechat_work.send_template_card",
		attribute.String("notification.channel", weChatWorkAppChannel),
		attribute.String("wechat_work.agent_id", agentID),
	)
	status := "failed"
	var sendErr error
	defer func() {
		span.SetAttributes(attribute.String("notification.status", status))
		metrics.NotificationsTotal.WithLabelValues(weChatWorkAppChannel, status).Inc()
		metrics.NotificationDuration.WithLabelValues(weChatWorkAppChannel, status).Observe(time.Since(startedAt).Seconds())
		observability.EndSpan(span, sendErr)
	}()

	if c == nil || c.httpClient == nil {
		sendErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_unavailable", "wechat work app client is unavailable", "notifier.wechat_work.send_template_card", true, nil)
		return WeChatWorkSendResult{}, sendErr
	}
	message.ToUser = strings.TrimSpace(message.ToUser)
	message.ToParty = strings.TrimSpace(message.ToParty)
	message.ToTag = strings.TrimSpace(message.ToTag)
	message.Title = truncateUTF8Bytes(strings.TrimSpace(message.Title), 128)
	message.Description = truncateUTF8Bytes(strings.TrimSpace(message.Description), 512)
	message.URL = strings.TrimSpace(message.URL)
	message.FallbackText = truncateUTF8Bytes(strings.TrimSpace(message.FallbackText), WeChatWorkTextByteLimit)
	span.SetAttributes(
		attribute.Bool("wechat_work.has_touser", message.ToUser != ""),
		attribute.Bool("wechat_work.has_toparty", message.ToParty != ""),
		attribute.Bool("wechat_work.has_totag", message.ToTag != ""),
		attribute.String("wechat_work.message_type", "template_card"),
	)
	if message.ToUser == "" && message.ToParty == "" && message.ToTag == "" {
		sendErr = domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_missing_recipient", "wechat work recipient is required", "notifier.wechat_work.send_template_card", false, nil)
		return WeChatWorkSendResult{}, sendErr
	}
	if message.Title == "" {
		sendErr = domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_empty_template_title", "wechat work template card title is required", "notifier.wechat_work.send_template_card", false, nil)
		return WeChatWorkSendResult{}, sendErr
	}
	if message.URL == "" {
		sendErr = domain.NewAppError(domain.ErrorKindInvalidInput, "wechat_work_empty_template_url", "wechat work template card url is required", "notifier.wechat_work.send_template_card", false, nil)
		return WeChatWorkSendResult{}, sendErr
	}

	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		sendErr = err
		return WeChatWorkSendResult{}, sendErr
	}

	requestBody := sendTemplateCardRequest{
		ToUser:  message.ToUser,
		ToParty: message.ToParty,
		ToTag:   message.ToTag,
		MsgType: "template_card",
		AgentID: c.agentID,
	}
	requestBody.TemplateCard.CardType = "text_notice"
	requestBody.TemplateCard.MainTitle.Title = message.Title
	requestBody.TemplateCard.MainTitle.Desc = message.Description
	requestBody.TemplateCard.CardAction.Type = 1
	requestBody.TemplateCard.CardAction.URL = message.URL
	for _, button := range message.Buttons {
		title := truncateUTF8Bytes(strings.TrimSpace(button.Text), 128)
		urlValue := strings.TrimSpace(button.URL)
		if title == "" || urlValue == "" {
			continue
		}
		requestBody.TemplateCard.JumpList = append(requestBody.TemplateCard.JumpList, struct {
			Type  int    `json:"type"`
			Title string `json:"title"`
			URL   string `json:"url"`
		}{Type: 1, Title: title, URL: urlValue})
	}
	if len(requestBody.TemplateCard.JumpList) == 0 {
		requestBody.TemplateCard.JumpList = append(requestBody.TemplateCard.JumpList, struct {
			Type  int    `json:"type"`
			Title string `json:"title"`
			URL   string `json:"url"`
		}{Type: 1, Title: "查看进度", URL: message.URL})
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		sendErr = err
		return WeChatWorkSendResult{}, sendErr
	}
	endpoint := c.apiBaseURL + "/cgi-bin/message/send?access_token=" + url.QueryEscape(accessToken)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		sendErr = err
		return WeChatWorkSendResult{}, sendErr
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpStartedAt := time.Now()
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		recordWeChatWorkExternalHTTPRequest(weChatWorkMessageSendOp, c.apiBaseURL, "error", time.Since(httpStartedAt))
		sendErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_send_failed", "wechat work template card send request failed", "notifier.wechat_work.send_template_card", true, err)
		return WeChatWorkSendResult{}, sendErr
	}
	defer httpResponse.Body.Close()
	httpStatus := strconv.Itoa(httpResponse.StatusCode)
	recordWeChatWorkExternalHTTPRequest(weChatWorkMessageSendOp, c.apiBaseURL, httpStatus, time.Since(httpStartedAt))
	span.SetAttributes(attribute.Int("http.response.status_code", httpResponse.StatusCode))
	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		sendErr = err
		return WeChatWorkSendResult{}, sendErr
	}
	var decoded sendTextResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		sendErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_invalid_response", "wechat work template card response is invalid", "notifier.wechat_work.send_template_card", true, err)
		return WeChatWorkSendResult{}, sendErr
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
	span.SetAttributes(attribute.Int("wechat_work.errcode", decoded.ErrCode))
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices || decoded.ErrCode != 0 {
		providerMessage := strings.TrimSpace(decoded.ErrMsg)
		if providerMessage == "" {
			providerMessage = http.StatusText(httpResponse.StatusCode)
		}
		sendErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_provider_error", providerMessage, "notifier.wechat_work.send_template_card", true, nil)
		return result, sendErr
	}
	status = "success"
	return result, nil
}

func (c *WeChatWorkAppClient) getAccessToken(ctx context.Context) (string, error) {
	ctx, span := observability.StartSpan(ctx, "notifier.wechat_work.get_token",
		attribute.String("notification.channel", weChatWorkAppChannel),
		attribute.String("http.request.host", apiHost(c.apiBaseURL)),
	)
	cacheHit := false
	var tokenErr error
	defer func() {
		span.SetAttributes(attribute.Bool("wechat_work.token_cache_hit", cacheHit))
		if tokenErr == nil {
			span.SetAttributes(attribute.String("wechat_work.token.status", "success"))
		} else {
			span.SetAttributes(attribute.String("wechat_work.token.status", "failed"))
		}
		observability.EndSpan(span, tokenErr)
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
		tokenErr = err
		return "", tokenErr
	}
	httpStartedAt := time.Now()
	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		recordWeChatWorkExternalHTTPRequest(weChatWorkGetTokenOp, c.apiBaseURL, "error", time.Since(httpStartedAt))
		tokenErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_token_failed", "wechat work token request failed", "notifier.wechat_work.token", true, err)
		return "", tokenErr
	}
	defer httpResponse.Body.Close()
	httpStatus := strconv.Itoa(httpResponse.StatusCode)
	recordWeChatWorkExternalHTTPRequest(weChatWorkGetTokenOp, c.apiBaseURL, httpStatus, time.Since(httpStartedAt))
	span.SetAttributes(attribute.Int("http.response.status_code", httpResponse.StatusCode))

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		tokenErr = err
		return "", tokenErr
	}
	var decoded tokenResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		tokenErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_token_invalid_response", "wechat work token response is invalid", "notifier.wechat_work.token", true, err)
		return "", tokenErr
	}
	span.SetAttributes(attribute.Int("wechat_work.errcode", decoded.ErrCode))
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices || decoded.ErrCode != 0 || decoded.AccessToken == "" {
		message := strings.TrimSpace(decoded.ErrMsg)
		if message == "" {
			message = http.StatusText(httpResponse.StatusCode)
		}
		tokenErr = domain.NewAppError(domain.ErrorKindUnavailable, "wechat_work_token_provider_error", message, "notifier.wechat_work.token", true, nil)
		return "", tokenErr
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

func recordWeChatWorkExternalHTTPRequest(operation string, apiBaseURL string, status string, duration time.Duration) {
	if status == "" {
		status = "unknown"
	}
	metrics.ExternalHTTPRequestsTotal.WithLabelValues(operation, apiHost(apiBaseURL), status).Inc()
	metrics.ExternalHTTPRequestDuration.WithLabelValues(operation, apiHost(apiBaseURL)).Observe(duration.Seconds())
}

func apiHost(apiBaseURL string) string {
	parsed, err := url.Parse(apiBaseURL)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}
	return parsed.Host
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
