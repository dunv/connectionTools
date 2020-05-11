package taskQueue

import (
	"context"
	"errors"
	"sync"
	"testing"
)

var taskQueueMockLock = &sync.Mutex{}

func successOnFirstTryFn(ctx context.Context) error {
	taskQueueMockLock.Lock()
	defer taskQueueMockLock.Unlock()

	// Check that all context is there
	_ = MaxRetryFromCtx(ctx)
	_ = BackoffInitialFromCtx(ctx)
	_ = BackoffFactorFromCtx(ctx)
	_ = BackoffLimitFromCtx(ctx)
	_ = CurrentRetryFromCtx(ctx)
	_ = CurrentBackoffFromCtx(ctx)
	_ = ctx.Value("testing").(*testing.T)

	return nil
}

var successOnSecondTry = 1

func successOnSecondTryFn(ctx context.Context) error {
	taskQueueMockLock.Lock()
	defer taskQueueMockLock.Unlock()

	// Check that all context is there
	_ = MaxRetryFromCtx(ctx)
	_ = MaxRetryFromCtx(ctx)
	_ = BackoffInitialFromCtx(ctx)
	_ = BackoffFactorFromCtx(ctx)
	_ = BackoffLimitFromCtx(ctx)
	_ = CurrentRetryFromCtx(ctx)
	_ = CurrentBackoffFromCtx(ctx)
	_ = ctx.Value("testing").(*testing.T)

	if successOnSecondTry < 2 {
		successOnSecondTry++
		return errors.New("send timeout")
	}
	return nil
}

var successOnThirdTry = 1

func successOnThirdTryFn(ctx context.Context) error {
	taskQueueMockLock.Lock()
	defer taskQueueMockLock.Unlock()

	// Check that all context is there
	_ = MaxRetryFromCtx(ctx)
	_ = MaxRetryFromCtx(ctx)
	_ = BackoffInitialFromCtx(ctx)
	_ = BackoffFactorFromCtx(ctx)
	_ = BackoffLimitFromCtx(ctx)
	_ = CurrentRetryFromCtx(ctx)
	_ = CurrentBackoffFromCtx(ctx)
	_ = ctx.Value("testing").(*testing.T)

	if successOnThirdTry < 3 {
		successOnThirdTry++
		return errors.New("send timeout")
	}
	return nil
}

var successOnTenthTry = 1

func successOnTenthTryFn(ctx context.Context) error {
	taskQueueMockLock.Lock()
	defer taskQueueMockLock.Unlock()

	// Check that all context is there
	_ = MaxRetryFromCtx(ctx)
	_ = MaxRetryFromCtx(ctx)
	_ = BackoffInitialFromCtx(ctx)
	_ = BackoffFactorFromCtx(ctx)
	_ = BackoffLimitFromCtx(ctx)
	_ = CurrentRetryFromCtx(ctx)
	_ = CurrentBackoffFromCtx(ctx)
	_ = ctx.Value("testing").(*testing.T)

	if successOnTenthTry < 10 {
		successOnTenthTry++
		return errors.New("send timeout")
	}
	return nil
}
