package util

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// Retry 首次成功：fn 仅执行一次。
func TestRetryFirstSuccess(t *testing.T) {
	var calls int32
	err := Retry(context.Background(), types.RetryPolicy{MaxAttempts: 3}, func() error {
		atomic.AddInt32(&calls, 1)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), calls)
}

// Retry 第二次成功：共 2 次调用。
func TestRetrySecondAttemptSuccess(t *testing.T) {
	var calls int32
	err := Retry(t.Context(), types.RetryPolicy{MaxAttempts: 3}, func() error {
		if atomic.AddInt32(&calls, 1) == 1 {
			return errors.New("transient")
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), calls)
}

// Retry 耗尽：返回最后一次错误，不再执行。
func TestRetryExhausted(t *testing.T) {
	poolErr := errors.New("always fail")
	var calls int32
	err := Retry(context.Background(), types.RetryPolicy{MaxAttempts: 2}, func() error {
		atomic.AddInt32(&calls, 1)
		return poolErr
	})
	require.ErrorIs(t, err, poolErr)
	require.Equal(t, int32(2), calls)
}

// Retry 策略零值 / MaxAttempts<=1：等价单次执行。
func TestRetryZeroPolicySingleAttempt(t *testing.T) {
	var calls int32
	err := Retry(context.Background(), types.RetryPolicy{}, func() error {
		atomic.AddInt32(&calls, 1)
		return errors.New("fail")
	})
	require.Error(t, err)
	require.Equal(t, int32(1), calls)
}

// Retry 退避递增并封顶；失败后立即重试（BaseBackoff<=0）时无等待。
func TestRetryBackoffCap(t *testing.T) {
	// MaxAttempts=4, base=1ms：无封顶时总等待 1+2+4=7ms；base=10s 封顶 20ms 时总等待极短。
	start := time.Now()
	_ = Retry(context.Background(), types.RetryPolicy{
		MaxAttempts: 4, BaseBackoff: 10 * time.Second, MaxBackoff: 20 * time.Millisecond,
	}, func() error { return errors.New("fail") })
	require.Less(t, time.Since(start), time.Second)

	// base=0：立即重试，4 次近乎瞬时完成。
	start = time.Now()
	_ = Retry(context.Background(), types.RetryPolicy{MaxAttempts: 4},
		func() error { return errors.New("fail") })
	require.Less(t, time.Since(start), 50*time.Millisecond)
}

// Retry 退避等待期间 ctx 取消：立即中断并返回最后一次错误。
func TestRetryCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int32
	start := time.Now()
	err := Retry(ctx, types.RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Minute},
		func() error {
			atomic.AddInt32(&calls, 1)
			cancel() // 第一次失败后取消
			return errors.New("first fail")
		})
	require.Error(t, err)
	require.Equal(t, int32(1), calls) // 不再发起第二次尝试
	require.Less(t, time.Since(start), time.Second)
}
