package taskQueue

import (
	"context"
	"time"
)

type TaskContextKey string

const (
	ctxKeyMaxRetries     TaskContextKey = "taskQueueOption.maxRetries"
	ctxKeyBackoffInitial TaskContextKey = "taskQueueOption.backoffInitial"
	ctxKeyBackoffFactor  TaskContextKey = "taskQueueOption.backoffFactor"
	ctxKeyBackoffLimit   TaskContextKey = "taskQueueOption.backoffLimit"
	ctxKeyCurrentRetry   TaskContextKey = "taskQueueOption.currentRetry"
	ctxKeyCurrentBackoff TaskContextKey = "taskQueueOption.currentBackoff"
)

func MaxRetryFromCtx(ctx context.Context) int {
	return ctx.Value(ctxKeyMaxRetries).(int)
}
func BackoffInitialFromCtx(ctx context.Context) time.Duration {
	return ctx.Value(ctxKeyBackoffInitial).(time.Duration)
}
func BackoffFactorFromCtx(ctx context.Context) int {
	return ctx.Value(ctxKeyBackoffFactor).(int)
}
func BackoffLimitFromCtx(ctx context.Context) time.Duration {
	return ctx.Value(ctxKeyBackoffLimit).(time.Duration)
}
func CurrentRetryFromCtx(ctx context.Context) int {
	return ctx.Value(ctxKeyCurrentRetry).(int)
}
func CurrentBackoffFromCtx(ctx context.Context) time.Duration {
	return ctx.Value(ctxKeyCurrentBackoff).(time.Duration)
}

func withContext(t taskQueueOptions, currentRetry int, currentBackoff time.Duration) context.Context {
	ctx := context.WithValue(t.ctx, ctxKeyMaxRetries, t.maxRetries)
	ctx = context.WithValue(ctx, ctxKeyBackoffInitial, t.backoffInitial)
	ctx = context.WithValue(ctx, ctxKeyBackoffFactor, t.backoffFactor)
	ctx = context.WithValue(ctx, ctxKeyBackoffLimit, t.backoffLimit)
	ctx = context.WithValue(ctx, ctxKeyCurrentRetry, currentRetry)
	ctx = context.WithValue(ctx, ctxKeyCurrentBackoff, currentBackoff)
	return ctx
}
