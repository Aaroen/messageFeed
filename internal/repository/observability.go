package repository

import (
	"context"
	"time"

	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"

	"go.opentelemetry.io/otel/attribute"
)

func traceRepositoryOperation(ctx context.Context, name string, operation string, table string) (context.Context, func(error)) {
	startedAt := time.Now()
	ctx, span := observability.StartSpan(ctx, name,
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", operation),
		attribute.String("db.sql.table", table),
	)
	return ctx, func(err error) {
		metrics.DatabaseQueriesTotal.WithLabelValues(operation, table).Inc()
		metrics.DatabaseQueryDuration.WithLabelValues(operation, table).Observe(time.Since(startedAt).Seconds())
		observability.EndSpan(span, err)
	}
}
