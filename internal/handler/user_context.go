package handler

import (
	"context"
	"net/http"

	"messagefeed/internal/domain"
	"messagefeed/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	defaultSingleUserID int64 = 1
	userIDKey                 = "user_id"
	authenticatedKey          = "authenticated"
	authUserKey               = "auth_user"
	authSessionKey            = "auth_session"
)

type authService interface {
	AuthenticateSession(ctx context.Context, rawToken string) (service.CurrentAuth, error)
	CookieName() string
}

func UserContext(authService authService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authService == nil {
			c.Set(userIDKey, defaultSingleUserID)
			c.Set(authenticatedKey, true)
			c.Next()
			return
		}
		cookie, err := c.Cookie(authService.CookieName())
		if err == nil && cookie != "" {
			auth, err := authService.AuthenticateSession(c.Request.Context(), cookie)
			if err == nil && auth.Authenticated {
				c.Set(userIDKey, auth.User.ID)
				c.Set(authenticatedKey, true)
				c.Set(authUserKey, auth.User)
				c.Set(authSessionKey, auth.Session)
			}
		}
		c.Next()
	}
}

func currentUserID(c *gin.Context) int64 {
	value, ok := c.Get(userIDKey)
	if !ok {
		return defaultSingleUserID
	}
	userID, ok := value.(int64)
	if !ok || userID < 1 {
		return defaultSingleUserID
	}
	return userID
}

func currentAuth(c *gin.Context) service.CurrentAuth {
	authenticated, _ := c.Get(authenticatedKey)
	if ok, _ := authenticated.(bool); !ok {
		return service.CurrentAuth{}
	}
	user, _ := c.Get(authUserKey)
	session, _ := c.Get(authSessionKey)
	authUser, _ := user.(domain.User)
	authSession, _ := session.(domain.UserSession)
	return service.CurrentAuth{
		Authenticated: true,
		User:          authUser,
		Session:       authSession,
	}
}

func requireAuth(authService authService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authService == nil {
			c.Next()
			return
		}
		authenticated, _ := c.Get(authenticatedKey)
		if ok, _ := authenticated.(bool); !ok {
			Error(c, http.StatusUnauthorized, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}
		c.Next()
	}
}
