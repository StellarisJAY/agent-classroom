package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// Config 应用配置，字段与 config.yaml 对应，支持环境变量覆盖
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	CORS      CORSConfig
	Crypto    CryptoConfig
	Model     ModelConfig
	Extractor ExtractorConfig
	Storage   StorageConfig
	Log       LogConfig
}

// ExtractorConfig 参考文档提取与 token 预算配置。
type ExtractorConfig struct {
	// Provider 提取链路选择：local 仅本地；mineru 强制外部；chain 外部优先本地兜底
	Provider string `mapstructure:"provider"`
	// MaxDocTokens 注入提示词的参考文档 token 总预算
	MaxDocTokens int `mapstructure:"max_doc_tokens"`
	// CharsPerToken token→字符换算系数（中文约 1.5，纯英文可设 2~4）
	CharsPerToken float64 `mapstructure:"chars_per_token"`
	// ExtractTimeout 单文档同步提取超时（大纲生成后台任务首步 EnsureDocsReady 用）
	ExtractTimeout time.Duration `mapstructure:"extract_timeout"`
	// Mineru minerU 外部提取服务配置
	Mineru MineruConfig `mapstructure:"mineru"`
}

// MineruConfig minerU 文档提取服务接入配置（官方 API 由官方 Go SDK 实现）。
type MineruConfig struct {
	// BaseURL 官方 API 或自托管服务的 v4 兼容端点（空 → SDK 默认 https://mineru.net/api/v4）
	BaseURL string `mapstructure:"base_url"`
	// AdminToken 官方 API 的管理令牌；为空或无效时回退免登录 Flash 提取（10MB/20 页限制）
	AdminToken string `mapstructure:"admin_token"`
	// Timeout 单篇文档提取（上传+轮询，SDK 内部托管）总超时
	Timeout time.Duration `mapstructure:"timeout"`
}

// StorageConfig 对象存储配置，type 决定使用哪种后端。
type StorageConfig struct {
	// Type 存储后端：local | minio
	Type string `mapstructure:"type"`
	// LocalDir 本地存储根目录（type=local 时生效）
	LocalDir string `mapstructure:"local_dir"`
	// Minio Minio 对象存储配置（type=minio 时生效）
	Minio MinioConfig `mapstructure:"minio"`
}

// MinioConfig Minio 对象存储连接配置。
type MinioConfig struct {
	// Endpoint Minio 服务地址，如 localhost:9000
	Endpoint string `mapstructure:"endpoint"`
	// AccessKey 访问密钥
	AccessKey string `mapstructure:"access_key"`
	// SecretKey 访问密钥
	SecretKey string `mapstructure:"secret_key"`
	// Bucket 存储桶名称
	Bucket string `mapstructure:"bucket"`
	// UseSSL 是否使用 HTTPS
	UseSSL bool `mapstructure:"use_ssl"`
	// Region 区域，默认 us-east-1
	Region string `mapstructure:"region"`
}

// CryptoConfig AES-256-GCM 加密配置，用于 API Key 等敏感字段存储加密。
type CryptoConfig struct {
	// EncryptionKey 32 字节 key
	EncryptionKey string `mapstructure:"encryption_key"`
}

// ModelConfig 模型相关配置。
type ModelConfig struct {
	// Options 平台全局可选模型清单，供创建课程时选择（用户不自行配置模型）。
	// 清单中 default: true 的项作为该用途的兜底模型（未选择课程模型时使用），每个 kind 至多一个。
	Options []ModelOptionConfig `mapstructure:"options"`
	// Timeout 非流式请求（大纲 / 环节内容生成 / 文生图）超时；思考模式 max 时自动放大 1.5 倍。
	Timeout time.Duration `mapstructure:"timeout"`
	// StreamTimeout 流式请求（问答）超时；0 表示不限制，由上层 context 控制。
	StreamTimeout time.Duration `mapstructure:"stream_timeout"`
	// Retry 生成类调用（大纲 / 环节内容 / 文生图）失败重试策略。
	Retry RetryConfig `mapstructure:"retry"`
}

// ModelOptionConfig 平台全局可选模型项。key 为唯一标识，随课程持久化以复用。
type ModelOptionConfig struct {
	// Key 唯一标识（前端选择与 course.model_key 持久化用）
	Key string `mapstructure:"key"`
	// Kind 用途：llm | image
	Kind string `mapstructure:"kind"`
	// Label 展示名；为空时回退 model
	Label string `mapstructure:"label"`
	// Default 是否为该用途的兜底模型（未选择课程模型时使用）；每个 kind 至多一个
	Default bool `mapstructure:"default"`
	// Provider 供应商标识
	Provider string `mapstructure:"provider"`
	// Model 模型名
	Model string `mapstructure:"model"`
	// BaseURL API 地址
	BaseURL string `mapstructure:"base_url"`
	// APIKey 访问密钥
	APIKey string `mapstructure:"api_key"`
}

// RetryConfig 生成调用失败重试配置（见 types.RetryPolicy）。
type RetryConfig struct {
	// MaxAttempts 总尝试次数（含首次）；<=1 等价不重试
	MaxAttempts int `mapstructure:"max_attempts"`
	// BaseBackoff 首次重试前的等待；<=0 表示失败后立即重试
	BaseBackoff time.Duration `mapstructure:"base_backoff"`
	// MaxBackoff 指数退避封顶；<=0 表示不封顶
	MaxBackoff time.Duration `mapstructure:"max_backoff"`
}

// Policy 换算为业务侧重试策略。
func (c RetryConfig) Policy() types.RetryPolicy {
	return types.RetryPolicy{
		MaxAttempts: c.MaxAttempts,
		BaseBackoff: c.BaseBackoff,
		MaxBackoff:  c.MaxBackoff,
	}
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	Timezone        string        `mapstructure:"timezone"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
	MaxAge         int      `mapstructure:"max_age"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// DSN 返回 postgres 连接串
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode, c.Timezone,
	)
}

// Load 读取配置：内置默认值 → config.yaml → 环境变量覆盖
func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", 10*time.Second)
	// write_timeout 覆盖 SSE 连接：讨论模式单流 loop 最多 8 轮 LLM 流式调用，
	// 默认取 15 分钟（远超 loop 时长上界）；如需收紧请评估讨论模式最长耗时。
	v.SetDefault("server.write_timeout", 15*time.Minute)

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.dbname", "agent_classroom")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.timezone", "Asia/Shanghai")
	v.SetDefault("database.max_open_conns", 20)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", time.Hour)

	v.SetDefault("jwt.secret", "change-me-to-a-32-byte-secret-key")
	v.SetDefault("jwt.expire_hours", 24)

	v.SetDefault("cors.max_age", 86400)

	v.SetDefault("crypto.encryption_key", "change-me-to-a-32-byte-encryption-key")

	v.SetDefault("model.timeout", 300*time.Second)
	v.SetDefault("model.stream_timeout", 0*time.Second)

	// 生成类调用重试：默认失败后立即重试一次（与历史 retryCall 行为一致）
	v.SetDefault("model.retry.max_attempts", 2)
	v.SetDefault("model.retry.base_backoff", 0*time.Second)
	v.SetDefault("model.retry.max_backoff", 0*time.Second)

	v.SetDefault("extractor.provider", "chain")
	v.SetDefault("extractor.max_doc_tokens", 20000)
	v.SetDefault("extractor.chars_per_token", 1.5)
	v.SetDefault("extractor.extract_timeout", 5*time.Minute)
	v.SetDefault("extractor.mineru.base_url", "")
	v.SetDefault("extractor.mineru.admin_token", "")
	v.SetDefault("extractor.mineru.timeout", 300*time.Second)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

	v.SetDefault("storage.type", "local")
	v.SetDefault("storage.local_dir", "./data/uploads")
	v.SetDefault("storage.minio.endpoint", "localhost:9000")
	v.SetDefault("storage.minio.access_key", "")
	v.SetDefault("storage.minio.secret_key", "")
	v.SetDefault("storage.minio.bucket", "agent-classroom")
	v.SetDefault("storage.minio.use_ssl", false)
	v.SetDefault("storage.minio.region", "us-east-1")

	// 读取同目录 config.yaml；未找到时不报错，走默认 + env
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./internal/config")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	// 环境变量覆盖：前缀 APP_，点号转下划线，如 APP_DATABASE_HOST、APP_JWT_SECRET
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
