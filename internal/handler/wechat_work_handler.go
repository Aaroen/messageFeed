package handler

import (
	"io"
	"net/http"

	"messagefeed/internal/channel/wechatwork"

	"github.com/gin-gonic/gin"
)

const maxWeChatWorkCallbackBodyBytes int64 = 1 << 20

type wechatWorkAppCallback interface {
	VerifyURL(input wechatwork.VerifyURLInput) (string, error)
	ParseInboundMessage(input wechatwork.ParseInboundMessageInput) (wechatwork.InboundMessage, error)
}

type wechatWorkHandler struct {
	appCallback wechatWorkAppCallback
}

func registerWeChatWorkRoutes(router *gin.RouterGroup, appCallback wechatWorkAppCallback) {
	handler := wechatWorkHandler{appCallback: appCallback}
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

	if _, err := h.appCallback.ParseInboundMessage(wechatwork.ParseInboundMessageInput{
		MsgSignature: c.Query("msg_signature"),
		Timestamp:    c.Query("timestamp"),
		Nonce:        c.Query("nonce"),
		Body:         body,
	}); err != nil {
		RenderError(c, err, "invalid wechat work callback message")
		return
	}

	c.String(http.StatusOK, "success")
}
