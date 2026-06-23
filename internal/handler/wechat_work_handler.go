package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"messagefeed/internal/channel/wechatwork"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
)

const maxWeChatWorkCallbackBodyBytes int64 = 1 << 20

type wechatWorkAppCallback interface {
	VerifyURL(input wechatwork.VerifyURLInput) (string, error)
	ParseInboundMessage(input wechatwork.ParseInboundMessageInput) (wechatwork.InboundMessage, error)
}

type wechatWorkInboundReceiver interface {
	ReceiveWeChatWorkAppMessage(ctx context.Context, input service.ReceiveWeChatWorkAppMessageInput) (service.ReceiveWeChatWorkAppMessageResult, error)
}

type wechatWorkHandler struct {
	appCallback     wechatWorkAppCallback
	inboundReceiver wechatWorkInboundReceiver
}

func registerWeChatWorkRoutes(router *gin.RouterGroup, appCallback wechatWorkAppCallback, inboundReceiver wechatWorkInboundReceiver) {
	handler := wechatWorkHandler{appCallback: appCallback, inboundReceiver: inboundReceiver}
	router.GET("/channels/wechat-work/app/callback", handler.verifyAppCallbackURL)
	router.POST("/channels/wechat-work/app/callback", handler.receiveAppCallback)
}

func (h wechatWorkHandler) verifyAppCallbackURL(c *gin.Context) {
	startedAt := time.Now()
	status := "success"
	ctx, span := observability.StartSpan(c.Request.Context(), "handler.wechat_work.verify_callback",
		attribute.String("wechat_work.operation", "verify_url"),
	)
	c.Request = c.Request.WithContext(ctx)
	var opErr error
	defer func() {
		span.SetAttributes(attribute.String("wechat_work.callback.status", status))
		metrics.WeChatWorkCallbacksTotal.WithLabelValues("verify_url", status).Inc()
		metrics.WeChatWorkCallbackDuration.WithLabelValues("verify_url", status).Observe(time.Since(startedAt).Seconds())
		observability.EndSpan(span, opErr)
	}()

	if h.appCallback == nil {
		status = "unavailable"
		opErr = errors.New("wechat work callback unavailable")
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "wechat work callback unavailable")
		return
	}

	echo, err := h.appCallback.VerifyURL(wechatwork.VerifyURLInput{
		MsgSignature: c.Query("msg_signature"),
		Timestamp:    c.Query("timestamp"),
		Nonce:        c.Query("nonce"),
		EchoStr:      c.Query("echostr"),
	})
	if err != nil {
		status = "failed"
		opErr = err
		RenderError(c, err, "invalid wechat work callback verification")
		return
	}

	c.String(http.StatusOK, echo)
}

func (h wechatWorkHandler) receiveAppCallback(c *gin.Context) {
	startedAt := time.Now()
	status := "success"
	ctx, span := observability.StartSpan(c.Request.Context(), "handler.wechat_work.receive_callback",
		attribute.String("wechat_work.operation", "receive_message"),
	)
	c.Request = c.Request.WithContext(ctx)
	var opErr error
	defer func() {
		span.SetAttributes(attribute.String("wechat_work.callback.status", status))
		metrics.WeChatWorkCallbacksTotal.WithLabelValues("receive_message", status).Inc()
		metrics.WeChatWorkCallbackDuration.WithLabelValues("receive_message", status).Observe(time.Since(startedAt).Seconds())
		observability.EndSpan(span, opErr)
	}()

	if h.appCallback == nil {
		status = "unavailable"
		opErr = errors.New("wechat work callback unavailable")
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "wechat work callback unavailable")
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWeChatWorkCallbackBodyBytes+1))
	if err != nil {
		status = "failed"
		opErr = err
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	span.SetAttributes(attribute.Int("http.request.body.size", len(body)))
	if int64(len(body)) > maxWeChatWorkCallbackBodyBytes {
		status = "failed"
		opErr = errors.New("wechat work callback body too large")
		Error(c, http.StatusRequestEntityTooLarge, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}

	message, err := h.appCallback.ParseInboundMessage(wechatwork.ParseInboundMessageInput{
		MsgSignature: c.Query("msg_signature"),
		Timestamp:    c.Query("timestamp"),
		Nonce:        c.Query("nonce"),
		Body:         body,
	})
	if err != nil {
		status = "failed"
		opErr = err
		RenderError(c, err, "invalid wechat work callback message")
		return
	}
	span.SetAttributes(
		attribute.String("message.provider", message.Provider),
		attribute.String("message.type", message.MsgType),
		attribute.String("message.chat_type", message.ChatType),
	)
	if h.inboundReceiver != nil {
		if _, err := h.inboundReceiver.ReceiveWeChatWorkAppMessage(c.Request.Context(), service.ReceiveWeChatWorkAppMessageInput{
			Provider:          message.Provider,
			ProviderMessageID: message.ProviderMessageID,
			CorpID:            message.CorpID,
			AgentID:           message.AgentID,
			ExternalUserID:    message.ExternalUserID,
			ChatID:            message.ChatID,
			ChatType:          message.ChatType,
			MsgType:           message.MsgType,
			TextContent:       message.TextContent,
			EventType:         message.EventType,
			EventKey:          message.EventKey,
			RawXML:            message.RawXML,
			RequestID:         requestID(c),
			TraceID:           observability.TraceID(c.Request.Context()),
		}); err != nil {
			status = "failed"
			opErr = err
			RenderError(c, err, "wechat work callback processing failed")
			return
		}
	}

	c.String(http.StatusOK, "success")
}
