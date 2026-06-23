package handler

import "github.com/gin-gonic/gin"

const (
	defaultSingleUserID int64 = 1
	userIDKey                 = "user_id"
)

func UserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(userIDKey, defaultSingleUserID)
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
