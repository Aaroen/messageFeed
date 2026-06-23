package handler

import (
	"context"
	"io"
	"net/http"

	"messagefeed/internal/channel/wechatwork"
	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
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
	if h.appCallback == nil {
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
		RenderError(c, err, "invalid wechat work callback verification")
		return
	}

	c.String(http.StatusOK, echo)
}

func (h wechatWorkHandler) receiveAppCallback(c *gin.Context) {
	if h.appCallback == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "wechat work callback unavailable")
		return
	}

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWeChatWorkCallbackBodyBytes+1))
	if err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	if int64(len(body)) > maxWeChatWorkCallbackBodyBytes {
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
		RenderError(c, err, "invalid wechat work callback message")
		return
	}
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
		}); err != nil {
			RenderError(c, err, "wechat work callback processing failed")
			return
		}
	}

	c.String(http.StatusOK, "success")
}
