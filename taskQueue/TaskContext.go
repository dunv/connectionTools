package taskQueue

import (
	"context"
	"time"
)

const (
	ctxKeyMaxRetries     string = "taskQueueOption.maxRetries"
	ctxKeyBackoffInitial string = "taskQueueOption.backoffInitial"
	ctxKeyBackoffFactor  string = "taskQueueOption.backoffFactor"
	ctxKeyBackoffLimit   string = "taskQueueOption.backoffLimit"
	ctxKeyCurrentRetry   string = "taskQueueOption.currentRetry"
	ctxKeyCurrentBackoff string = "taskQueueOption.currentBackoff"
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
