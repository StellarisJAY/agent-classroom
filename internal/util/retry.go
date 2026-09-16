package util

import (
	"context"
	"time"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// Retry 通用重试：限制总尝试次数，重试前按指数退避等待
// （base、2·base、4·base…，封顶 MaxBackoff）。fn 返回 nil 即成功；
// 全部尝试耗尽返回最后一次错误；退避等待期间响应 ctx 取消，
// 立即中断并返回最后一次错误。MaxAttempts<=1 等价单次执行，
// BaseBackoff<=0 表示失败后立即重试。
func Retry(ctx context.Context, policy types.RetryPolicy, fn func() error) error {
	attempts := max(policy.MaxAttempts, 1)
	backoff := policy.BaseBackoff
	var last error
	for i := range attempts {
		if err := fn(); err == nil {
			return nil
		} else {
			last = err
		}
		if i == attempts-1 {
			return last
		}
		if policy.MaxBackoff > 0 {
			backoff = min(backoff, policy.MaxBackoff)
		}
		if backoff <= 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return last
		case <-time.After(backoff):
		}
		backoff *= 2
		if policy.MaxBackoff > 0 {
			backoff = min(backoff, policy.MaxBackoff)
		}
	}
	return last
}
