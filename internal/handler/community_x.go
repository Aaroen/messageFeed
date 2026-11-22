package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

type XBrowserProxies map[int64]*httputil.ReverseProxy

func NewXBrowserProxies(endpoints map[int64]*url.URL) XBrowserProxies {
	proxies := make(XBrowserProxies, len(endpoints))
	for userID, endpoint := range endpoints {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		transport.ResponseHeaderTimeout = 10 * time.Second
		proxies[userID] = &httputil.ReverseProxy{
			Transport: transport,
			Rewrite: func(request *httputil.ProxyRequest) {
				request.SetURL(endpoint)
				request.Out.Header.Del("Cookie")
				request.Out.Header.Del("Authorization")
				request.Out.Header.Del("Referer")
				request.Out.Header.Del("Origin")
			},
			ModifyResponse: func(response *http.Response) error {
				if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
					return errors.New("X 浏览器返回了意外的重定向")
				}
				response.Header.Del("Set-Cookie")
				response.Header.Del("Access-Control-Allow-Origin")
				response.Header.Set("Cache-Control", "no-store")
				response.Header.Set("X-Content-Type-Options", "nosniff")
				response.Header.Set("X-Frame-Options", "SAMEORIGIN")
				response.Header.Add("Content-Security-Policy", "frame-ancestors 'self'; object-src 'none'; base-uri 'none'; form-action 'none'")
				return nil
			},
			ErrorHandler: func(writer http.ResponseWriter, _ *http.Request, _ error) {
				http.Error(writer, "暂时无法连接 X 浏览器", http.StatusBadGateway)
			},
		}
	}
	return proxies
}

func registerXBrowserRoutes(router *gin.RouterGroup, connector *service.XConnector, sources *service.SourceService) {
	proxies := NewXBrowserProxies(connector.BrowserEndpoints)
	browser := router.Group("/community/x", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if site := c.GetHeader("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if proxies[currentUserID(c)] == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	})
	browser.GET("", func(c *gin.Context) {
		connected, err := connector.Connected(c.Request.Context(), currentUserID(c))
		if err != nil {
			respondXError(c, err)
			return
		}
		Success(c, gin.H{"connected": connected})
	})
	browser.POST("/input", func(c *gin.Context) {
		var input struct {
			Text string `json:"text"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
		if c.ShouldBindJSON(&input) != nil {
			Error(c, http.StatusBadRequest, http.StatusBadRequest, "输入内容无效")
			return
		}
		if err := connector.InsertText(c.Request.Context(), currentUserID(c), input.Text); err != nil {
			respondXError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	browser.POST("/subscribe", func(c *gin.Context) {
		var input struct {
			Account string `json:"account"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
		if c.ShouldBindJSON(&input) != nil {
			Error(c, http.StatusBadRequest, http.StatusBadRequest, "X 用户名无效")
			return
		}
		result, err := sources.SubscribeX(c.Request.Context(), connector, currentUserID(c), input.Account)
		if err != nil {
			respondXError(c, err)
			return
		}
		Success(c, gin.H{"source": sourceResponseFromDomain(result.Source), "item_count": result.ItemCount})
	})
	browser.GET("/browser/*asset", func(c *gin.Context) {
		asset := c.Param("asset")
		if path.Clean(asset) != asset || strings.Contains(asset, "\\") {
			c.Status(http.StatusNotFound)
			return
		}
		upgrade := c.GetHeader("Upgrade")
		if upgrade != "" {
			origin, err := url.Parse(c.GetHeader("Origin"))
			if !strings.EqualFold(upgrade, "websocket") || err != nil || origin.User != nil ||
				origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" ||
				!strings.EqualFold(origin.Host, c.Request.Host) || (origin.Scheme != "https" && origin.Scheme != "http") {
				c.Status(http.StatusForbidden)
				return
			}
		}
		duration := 20 * time.Second
		if upgrade != "" {
			duration = 60 * time.Minute
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), duration)
		defer cancel()
		request := c.Request.Clone(ctx)
		request.URL.Path = asset
		request.URL.RawPath = ""
		proxies[currentUserID(c)].ServeHTTP(c.Writer, request)
	})
}

func respondXError(c *gin.Context, err error) {
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrXAuthorization):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, service.ErrXInputFocus), errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	}
	Error(c, status, status, err.Error())
}
