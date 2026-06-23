package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"messagefeed/internal/domain"
	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

type authEndpointService interface {
	authService
	LocalLogin(ctx context.Context, input service.LocalLoginInput) (service.AuthSessionResult, error)
	Logout(ctx context.Context, rawToken string) error
	Me(ctx context.Context, auth service.CurrentAuth) (service.AuthMeResult, error)
	BuildWeChatWorkOAuthURL(ctx context.Context, input service.WeChatWorkOAuthURLInput) (service.WeChatWorkOAuthURLResult, error)
	HandleWeChatWorkOAuthCallback(ctx context.Context, input service.WeChatWorkOAuthCallbackInput) (service.WeChatWorkOAuthCallbackResult, error)
	ListBindings(ctx context.Context, userID int64) ([]service.AuthBindingResponse, error)
	DisableBinding(ctx context.Context, userID int64, accountID int64) (service.AuthBindingResponse, error)
	CookieMaxAge() int
	CookieSecure() bool
}

type authHandler struct {
	service authEndpointService
}

type localLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authLoginResponse struct {
	User      *service.AuthUserResponse `json:"user"`
	ExpiresAt string                    `json:"expires_at"`
}

func registerAuthRoutes(router *gin.RouterGroup, authService authEndpointService) {
	handler := authHandler{service: authService}
	auth := router.Group("/auth")
	auth.GET("/me", handler.me)
	auth.POST("/login", handler.login)
	auth.POST("/logout", handler.logout)
	auth.GET("/wechat-work/oauth-url", requireAuth(authService), handler.wechatWorkOAuthURL)
	auth.GET("/wechat-work/callback", handler.wechatWorkCallback)
	auth.GET("/bindings", requireAuth(authService), handler.listBindings)
	auth.POST("/bindings/:id/disable", requireAuth(authService), handler.disableBinding)
}

func (h authHandler) me(c *gin.Context) {
	if h.service == nil {
		Success(c, service.AuthMeResult{Authenticated: true, LoginEnabled: false, Bindings: []service.AuthBindingResponse{}})
		return
	}
	result, err := h.service.Me(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load current user failed")
		return
	}
	Success(c, result)
}

func (h authHandler) login(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	var request localLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.LocalLogin(c.Request.Context(), service.LocalLoginInput{
		Username:      request.Username,
		Password:      request.Password,
		UserAgent:     c.GetHeader("User-Agent"),
		RemoteAddress: c.ClientIP(),
	})
	if err != nil {
		RenderError(c, err, "login failed")
		return
	}
	setSessionCookie(c, h.service, result.Token, result.ExpiresAt)
	Success(c, authLoginResponse{
		User:      serviceUserResponse(result.User),
		ExpiresAt: result.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h authHandler) logout(c *gin.Context) {
	if h.service == nil {
		Success(c, gin.H{"logged_out": true})
		return
	}
	cookie, _ := c.Cookie(h.service.CookieName())
	if err := h.service.Logout(c.Request.Context(), cookie); err != nil {
		RenderError(c, err, "logout failed")
		return
	}
	clearSessionCookie(c, h.service)
	Success(c, gin.H{"logged_out": true})
}

func (h authHandler) wechatWorkOAuthURL(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	result, err := h.service.BuildWeChatWorkOAuthURL(c.Request.Context(), service.WeChatWorkOAuthURLInput{
		UserID:       currentUserID(c),
		Purpose:      c.DefaultQuery("purpose", "bind"),
		RedirectPath: c.DefaultQuery("redirect", "/settings"),
	})
	if err != nil {
		RenderError(c, err, "build wechat work oauth url failed")
		return
	}
	Success(c, result)
}

func (h authHandler) wechatWorkCallback(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	result, err := h.service.HandleWeChatWorkOAuthCallback(c.Request.Context(), service.WeChatWorkOAuthCallbackInput{
		Code:          c.Query("code"),
		State:         c.Query("state"),
		UserAgent:     c.GetHeader("User-Agent"),
		RemoteAddress: c.ClientIP(),
	})
	if err != nil {
		RenderError(c, err, "wechat work oauth callback failed")
		return
	}
	setSessionCookie(c, h.service, result.Token, result.ExpiresAt)
	redirectPath := appendOAuthResult(result.RedirectPath, "wechat_bound", "1")
	c.Redirect(http.StatusFound, redirectPath)
}

func (h authHandler) listBindings(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	result, err := h.service.ListBindings(c.Request.Context(), currentUserID(c))
	if err != nil {
		RenderError(c, err, "load bindings failed")
		return
	}
	Success(c, result)
}

func (h authHandler) disableBinding(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid binding id")
		return
	}
	result, err := h.service.DisableBinding(c.Request.Context(), currentUserID(c), accountID)
	if err != nil {
		RenderError(c, err, "disable binding failed")
		return
	}
	Success(c, result)
}

func setSessionCookie(c *gin.Context, service authEndpointService, token string, expiresAt time.Time) {
	cookie := &http.Cookie{
		Name:     service.CookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   service.CookieMaxAge(),
		Expires:  expiresAt.UTC(),
		HttpOnly: true,
		Secure:   service.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(c.Writer, cookie)
}

func clearSessionCookie(c *gin.Context, service authEndpointService) {
	cookie := &http.Cookie{
		Name:     service.CookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		Secure:   service.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(c.Writer, cookie)
}

func serviceUserResponse(user domain.User) *service.AuthUserResponse {
	return &service.AuthUserResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        string(user.Role),
		Status:      string(user.Status),
	}
}

func appendOAuthResult(path string, key string, value string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/settings"
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + key + "=" + value
}
