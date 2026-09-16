package types

import "time"

// RetryPolicy 生成调用（LLM / 文生图）失败重试策略。
// MaxAttempts<=1 表示不重试（单次尝试）；BaseBackoff<=0 表示失败后立即重试。
type RetryPolicy struct {
	// MaxAttempts 总尝试次数（含首次）
	MaxAttempts int `mapstructure:"max_attempts"`
	// BaseBackoff 首次重试前的等待时长，之后按指数递增
	BaseBackoff time.Duration `mapstructure:"base_backoff"`
	// MaxBackoff 退避上限封顶；<=0 表示不封顶
	MaxBackoff time.Duration `mapstructure:"max_backoff"`
}
