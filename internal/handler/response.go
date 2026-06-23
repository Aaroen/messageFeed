package handler

import (
	"errors"
	"net/http"

	"messagefeed/internal/domain"
	"messagefeed/internal/observability"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "request_id"
const errorCodeKey = "error_code"
const errorMessageKey = "error_message"

// APIResponse 是业务 API 的统一响应结构。
// 健康检查、指标和运行时节点端点保持原有响应形态，业务端点后续统一使用该结构。
type APIResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
}

// Success 返回业务成功响应。
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, APIResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: requestID(c),
		TraceID:   observability.TraceID(c.Request.Context()),
	})
}

// Created 返回业务创建成功响应。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, APIResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: requestID(c),
		TraceID:   observability.TraceID(c.Request.Context()),
	})
}

// Error 返回业务错误响应。
func Error(c *gin.Context, statusCode int, code int, message string) {
	c.Set(errorCodeKey, code)
	c.Set(errorMessageKey, message)
	c.JSON(statusCode, APIResponse{
		Code:      code,
		Message:   message,
		RequestID: requestID(c),
		TraceID:   observability.TraceID(c.Request.Context()),
	})
}

func RenderError(c *gin.Context, err error, fallbackMessage string) {
	statusCode, code, message := mapError(err, fallbackMessage)
	Error(c, statusCode, code, message)
}

func mapError(err error, fallbackMessage string) (int, int, string) {
	if fallbackMessage == "" {
		fallbackMessage = "operation failed"
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		message := appErr.Message
		if message == "" {
			message = fallbackMessage
		}
		return statusCodeForKind(appErr.Kind), codeForKind(appErr.Kind), message
	}

	switch domain.ClassifyError(err) {
	case domain.ErrorKindInvalidInput:
		return http.StatusBadRequest, http.StatusBadRequest, fallbackMessage
	case domain.ErrorKindNotFound:
		return http.StatusNotFound, http.StatusNotFound, fallbackMessage
	case domain.ErrorKindConflict:
		return http.StatusConflict, http.StatusConflict, fallbackMessage
	case domain.ErrorKindRateLimited:
		return http.StatusTooManyRequests, http.StatusTooManyRequests, fallbackMessage
	case domain.ErrorKindUnavailable:
		return http.StatusServiceUnavailable, http.StatusServiceUnavailable, fallbackMessage
	default:
		return http.StatusInternalServerError, http.StatusInternalServerError, fallbackMessage
	}
}

func statusCodeForKind(kind domain.ErrorKind) int {
	switch kind {
	case domain.ErrorKindInvalidInput:
		return http.StatusBadRequest
	case domain.ErrorKindNotFound:
		return http.StatusNotFound
	case domain.ErrorKindConflict:
		return http.StatusConflict
	case domain.ErrorKindRateLimited:
		return http.StatusTooManyRequests
	case domain.ErrorKindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func codeForKind(kind domain.ErrorKind) int {
	return statusCodeForKind(kind)
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
