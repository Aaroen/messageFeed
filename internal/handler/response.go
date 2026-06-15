package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "request_id"

// APIResponse 是业务 API 的统一响应结构。
// 健康检查、指标和运行时节点端点保持原有响应形态，业务端点后续统一使用该结构。
type APIResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Success 返回业务成功响应。
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, APIResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: requestID(c),
	})
}

// Error 返回业务错误响应。
func Error(c *gin.Context, statusCode int, code int, message string) {
	c.JSON(statusCode, APIResponse{
		Code:      code,
		Message:   message,
		RequestID: requestID(c),
	})
}

func requestID(c *gin.Context) string {
	value, ok := c.Get(requestIDKey)
	if !ok {
		return ""
	}
	requestID, ok := value.(string)
	if !ok {
		return ""
	}
	return requestID
}
