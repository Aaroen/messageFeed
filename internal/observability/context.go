package observability

import "context"

type requestIDContextKey struct{}

// WithRequestID 将请求标识写入标准 context，供 service、repository、fetcher 等层读取。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestID 从标准 context 中读取请求标识。
func RequestID(ctx context.Context) string {
	value, ok := ctx.Value(requestIDContextKey{}).(string)
	if !ok {
		return ""
	}
	return value
}
