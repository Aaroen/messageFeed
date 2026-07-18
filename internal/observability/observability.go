package observability

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"time"

	"messagefeed/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const TracerName = "messagefeed"

// ShutdownFunc 释放观测系统后台资源。
type ShutdownFunc func(context.Context) error

// NewLogger 创建生产可采集的结构化 JSON logger。
func NewLogger(writer io.Writer, level slog.Level, cfg config.Config) *slog.Logger {
	if writer == nil {
		writer = os.Stdout
	}
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
	})).With(
		"service", cfg.Observability.ServiceName,
		"service_version", cfg.Observability.ServiceVersion,
		"environment", cfg.Observability.Environment,
		"node_id", cfg.Runtime.AppNodeID,
		"deployment_mode", cfg.Runtime.DeploymentMode,
		"app_role", cfg.Runtime.AppRole,
	)
}

// InitTracing 初始化 OpenTelemetry trace provider。
func InitTracing(ctx context.Context, cfg config.ObservabilityConfig, nodeID string) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.TraceEnabled {
		return func(context.Context) error { return nil }, nil
	}

	exporterOptions := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
	}
	if cfg.OTLPInsecure {
		exporterOptions = append(exporterOptions, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(ctx, exporterOptions...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("service.instance.id", nodeID),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
	)
	otel.SetTracerProvider(tracerProvider)

	return tracerProvider.Shutdown, nil
}

// StartSpan 建立命名 span，并统一使用项目 tracer。
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(TracerName).Start(ctx, name, trace.WithAttributes(attrs...))
}

// EndSpan 根据错误状态结束 span。
func EndSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// TraceID 返回当前上下文中的 trace id。
func TraceID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.HasTraceID() {
		return ""
	}
	return spanContext.TraceID().String()
}

// SpanID 返回当前上下文中的 span id。
func SpanID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.HasSpanID() {
		return ""
	}
	return spanContext.SpanID().String()
}

// ShutdownWithTimeout 用统一超时关闭观测后台资源。
func ShutdownWithTimeout(ctx context.Context, shutdown ShutdownFunc, timeout time.Duration) error {
	if shutdown == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
