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
	RegisterWithInvite(ctx context.Context, input service.RegisterWithInviteInput) (service.AuthSessionResult, error)
	ChangePassword(ctx context.Context, input service.ChangePasswordInput) (service.AuthUserResponse, error)
	VerifyCurrentPassword(ctx context.Context, auth service.CurrentAuth, currentPassword string) error
	UpdateProfile(ctx context.Context, input service.UpdateProfileInput) (service.UserProfileResponse, error)
	ListSessions(ctx context.Context, auth service.CurrentAuth) ([]service.UserSessionResponse, error)
	RevokeSession(ctx context.Context, auth service.CurrentAuth, sessionID int64) error
	DeactivateAccount(ctx context.Context, input service.DeactivateAccountInput) error
	Logout(ctx context.Context, rawToken string) error
	Me(ctx context.Context, auth service.CurrentAuth) (service.AuthMeResult, error)
	GetUserContext(ctx context.Context, auth service.CurrentAuth) (service.UserContextResult, error)
	BuildWeChatWorkOAuthURL(ctx context.Context, input service.WeChatWorkOAuthURLInput) (service.WeChatWorkOAuthURLResult, error)
	HandleWeChatWorkOAuthCallback(ctx context.Context, input service.WeChatWorkOAuthCallbackInput) (service.WeChatWorkOAuthCallbackResult, error)
	ListBindings(ctx context.Context, userID int64) ([]service.AuthBindingResponse, error)
	DisableBinding(ctx context.Context, userID int64, accountID int64) (service.AuthBindingResponse, error)
	CreateInvite(ctx context.Context, input service.CreateInviteInput) (service.CreateInviteResult, error)
	ListInvites(ctx context.Context, auth service.CurrentAuth) ([]service.InviteCodeResponse, error)
	RevokeInvite(ctx context.Context, auth service.CurrentAuth, inviteID int64) (service.InviteCodeResponse, error)
	ListUsers(ctx context.Context, auth service.CurrentAuth) ([]service.AdminUserResponse, error)
	DeactivateUser(ctx context.Context, auth service.CurrentAuth, userID int64) (service.AdminUserResponse, error)
	RestoreUser(ctx context.Context, auth service.CurrentAuth, userID int64) (service.AdminUserResponse, error)
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

type registerRequest struct {
	InviteCode  string `json:"invite_code"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type updateProfileRequest struct {
	DisplayName            string   `json:"display_name"`
	Email                  string   `json:"email"`
	TimeZone               string   `json:"timezone"`
	Language               string   `json:"language"`
	Region                 string   `json:"region"`
	Bio                    string   `json:"bio"`
	FocusTopics            []string `json:"focus_topics"`
	BlockedTopics          []string `json:"blocked_topics"`
	MarketFocus            []string `json:"market_focus"`
	InstrumentFocus        []string `json:"instrument_focus"`
	RiskPreference         string   `json:"risk_preference"`
	NotificationQuietHours string   `json:"notification_quiet_hours"`
	AgentNotes             string   `json:"agent_notes"`
	ReplyStyle             string   `json:"reply_style"`
}

type deactivateAccountRequest struct {
	CurrentPassword string `json:"current_password"`
}

type createInviteRequest struct {
	Role       string `json:"role"`
	MaxUses    int    `json:"max_uses"`
	TTLSeconds int64  `json:"ttl_seconds"`
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
	auth.POST("/register", handler.register)
	auth.POST("/logout", handler.logout)
	auth.POST("/password", requireAuth(authService), handler.changePassword)
	auth.GET("/profile", requireAuth(authService), handler.profile)
	auth.PATCH("/profile", requireAuth(authService), handler.updateProfile)
	auth.GET("/sessions", requireAuth(authService), handler.listSessions)
	auth.DELETE("/sessions/:id", requireAuth(authService), handler.revokeSession)
	auth.DELETE("/account", requireAuth(authService), handler.deactivateAccount)
	auth.GET("/context", requireAuth(authService), handler.userContext)
	auth.GET("/wechat-work/oauth-url", requireAuth(authService), handler.wechatWorkOAuthURL)
	auth.GET("/wechat-work/callback", handler.wechatWorkCallback)
	auth.GET("/bindings", requireAuth(authService), handler.listBindings)
	auth.POST("/bindings/:id/disable", requireAuth(authService), handler.disableBinding)
	admin := router.Group("/admin")
	admin.GET("/invites", requireAuth(authService), handler.listInvites)
	admin.POST("/invites", requireAuth(authService), handler.createInvite)
	admin.DELETE("/invites/:id", requireAuth(authService), handler.revokeInvite)
	admin.GET("/users", requireAuth(authService), handler.listUsers)
	admin.DELETE("/users/:id", requireAuth(authService), handler.deactivateUser)
	admin.POST("/users/:id/restore", requireAuth(authService), handler.restoreUser)
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

func (h authHandler) register(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.RegisterWithInvite(c.Request.Context(), service.RegisterWithInviteInput{
		InviteCode:    request.InviteCode,
		Username:      request.Username,
		Password:      request.Password,
		DisplayName:   request.DisplayName,
		Email:         request.Email,
		UserAgent:     c.GetHeader("User-Agent"),
		RemoteAddress: c.ClientIP(),
	})
	if err != nil {
		RenderError(c, err, "register failed")
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

func (h authHandler) changePassword(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	var request changePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.ChangePassword(c.Request.Context(), service.ChangePasswordInput{
		UserID:          currentUserID(c),
		CurrentPassword: request.CurrentPassword,
		NewPassword:     request.NewPassword,
	})
	if err != nil {
		RenderError(c, err, "change password failed")
		return
	}
	Success(c, result)
}

func (h authHandler) profile(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	result, err := h.service.GetUserContext(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load profile failed")
		return
	}
	Success(c, result.Profile)
}

func (h authHandler) updateProfile(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	var request updateProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.UpdateProfile(c.Request.Context(), service.UpdateProfileInput{
		UserID:                 currentUserID(c),
		DisplayName:            request.DisplayName,
		Email:                  request.Email,
		TimeZone:               request.TimeZone,
		Language:               request.Language,
		Region:                 request.Region,
		Bio:                    request.Bio,
		FocusTopics:            request.FocusTopics,
		BlockedTopics:          request.BlockedTopics,
		MarketFocus:            request.MarketFocus,
		InstrumentFocus:        request.InstrumentFocus,
		RiskPreference:         request.RiskPreference,
		NotificationQuietHours: request.NotificationQuietHours,
		AgentNotes:             request.AgentNotes,
		ReplyStyle:             request.ReplyStyle,
	})
	if err != nil {
		RenderError(c, err, "update profile failed")
		return
	}
	Success(c, result)
}

func (h authHandler) listSessions(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	result, err := h.service.ListSessions(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load sessions failed")
		return
	}
	Success(c, result)
}

func (h authHandler) revokeSession(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || sessionID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid session id")
		return
	}
	if err := h.service.RevokeSession(c.Request.Context(), currentAuth(c), sessionID); err != nil {
		RenderError(c, err, "revoke session failed")
		return
	}
	if currentAuth(c).Session.ID == sessionID {
		clearSessionCookie(c, h.service)
	}
	Success(c, gin.H{"revoked": true})
}

func (h authHandler) deactivateAccount(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	var request deactivateAccountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.DeactivateAccount(c.Request.Context(), service.DeactivateAccountInput{
		UserID:          currentUserID(c),
		CurrentPassword: request.CurrentPassword,
	}); err != nil {
		RenderError(c, err, "delete account failed")
		return
	}
	clearSessionCookie(c, h.service)
	Success(c, gin.H{"deleted": true})
}

func (h authHandler) userContext(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	result, err := h.service.GetUserContext(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load user context failed")
		return
	}
	Success(c, result)
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

func (h authHandler) listInvites(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	result, err := h.service.ListInvites(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load invites failed")
		return
	}
	Success(c, result)
}

func (h authHandler) createInvite(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	var request createInviteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.CreateInvite(c.Request.Context(), service.CreateInviteInput{
		Creator:    currentAuth(c).User,
		Role:       request.Role,
		MaxUses:    request.MaxUses,
		TTLSeconds: request.TTLSeconds,
	})
	if err != nil {
		RenderError(c, err, "create invite failed")
		return
	}
	Created(c, result)
}

func (h authHandler) revokeInvite(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	inviteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || inviteID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid invite id")
		return
	}
	result, err := h.service.RevokeInvite(c.Request.Context(), currentAuth(c), inviteID)
	if err != nil {
		RenderError(c, err, "delete invite failed")
		return
	}
	Success(c, result)
}

func (h authHandler) listUsers(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	result, err := h.service.ListUsers(c.Request.Context(), currentAuth(c))
	if err != nil {
		RenderError(c, err, "load users failed")
		return
	}
	Success(c, result)
}

func (h authHandler) deactivateUser(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid user id")
		return
	}
	result, err := h.service.DeactivateUser(c.Request.Context(), currentAuth(c), userID)
	if err != nil {
		RenderError(c, err, "delete user failed")
		return
	}
	Success(c, result)
}

func (h authHandler) restoreUser(c *gin.Context) {
	if h.service == nil {
		Error(c, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "auth service unavailable")
		return
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID < 1 {
		Error(c, http.StatusBadRequest, http.StatusBadRequest, "invalid user id")
		return
	}
	result, err := h.service.RestoreUser(c.Request.Context(), currentAuth(c), userID)
	if err != nil {
		RenderError(c, err, "restore user failed")
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
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		Role:               string(user.Role),
		Status:             string(user.Status),
		PasswordConfigured: strings.TrimSpace(user.PasswordHash) != "",
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
