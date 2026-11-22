package handler

import (
	"errors"
	"net/http"

	"messagefeed/internal/domain"
	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

func registerWeChatRSSRoutes(router *gin.RouterGroup, provider *service.WeChatRSS, sources *service.SourceService) {
	connector := router.Group("/community/wechat", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		site := c.GetHeader("Sec-Fetch-Site")
		if site != "" && site != "same-origin" {
			Error(c, http.StatusForbidden, http.StatusForbidden, "请在订阅管理页面操作")
			c.Abort()
			return
		}
		if !provider.Available(currentUserID(c)) {
			Error(c, http.StatusNotFound, http.StatusNotFound, "当前用户未启用微信公众号连接")
			c.Abort()
			return
		}
		c.Next()
	})
	connector.GET("", func(c *gin.Context) {
		connected, err := provider.Connected(c.Request.Context(), currentUserID(c))
		respondWeChatRSS(c, gin.H{"connected": connected}, err)
	})
	connector.POST("/login", func(c *gin.Context) {
		qrCode, err := provider.StartLogin(c.Request.Context(), currentUserID(c))
		respondWeChatRSS(c, gin.H{"qr_code": qrCode}, err)
	})
	connector.GET("/login", func(c *gin.Context) {
		status, err := provider.LoginStatus(c.Request.Context(), currentUserID(c))
		respondWeChatRSS(c, status, err)
	})
	connector.POST("/login/otp", func(c *gin.Context) {
		var input struct {
			OTP string `json:"otp"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
		if err := c.ShouldBindJSON(&input); err != nil {
			Error(c, http.StatusBadRequest, http.StatusBadRequest, "验证码请求格式无效")
			return
		}
		err := provider.SubmitLoginOTP(c.Request.Context(), currentUserID(c), input.OTP)
		respondWeChatRSS(c, gin.H{"submitted": err == nil}, err)
	})
	connector.GET("/publishers", func(c *gin.Context) {
		publishers, err := provider.Publishers(c.Request.Context(), currentUserID(c))
		respondWeChatRSS(c, publishers, err)
	})
	connector.POST("/publishers/:id/articles", func(c *gin.Context) {
		articles, err := provider.Articles(c.Request.Context(), currentUserID(c), c.Param("id"))
		respondWeChatRSS(c, articles, err)
	})
	connector.POST("/publishers/:id/subscribe", func(c *gin.Context) {
		result, err := sources.SubscribeWeChatPublisher(c.Request.Context(), provider, currentUserID(c), c.Param("id"))
		respondWeChatRSS(c, gin.H{"source_id": result.Source.ID, "item_count": result.ItemCount}, err)
	})
}

func respondWeChatRSS(c *gin.Context, data any, err error) {
	if err != nil {
		status := http.StatusBadGateway
		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			status = http.StatusBadRequest
		case errors.Is(err, domain.ErrNotFound):
			status = http.StatusNotFound
		}
		Error(c, status, status, err.Error())
		return
	}
	Success(c, data)
}
