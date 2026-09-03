package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置，字段与 config.yaml 对应，支持环境变量覆盖
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	CORS     CORSConfig
	Crypto   CryptoConfig
	Model    ModelConfig
	Storage  StorageConfig
	Log      LogConfig
}

// StorageConfig 对象存储配置（本期仅本地磁盘）。
type StorageConfig struct {
	// LocalDir 本地存储根目录
	LocalDir string `mapstructure:"local_dir"`
}

// CryptoConfig AES-256-GCM 加密配置，用于 API Key 等敏感字段存储加密。
type CryptoConfig struct {
	// EncryptionKey 32 字节 key
	EncryptionKey string `mapstructure:"encryption_key"`
}

// ModelConfig 模型相关配置。
type ModelConfig struct {
	// Default 服务端统一兜底模型，用户无专属配置时使用。
	Default ModelDefaultConfig `mapstructure:"default"`
}

// ModelDefaultConfig 兜底模型配置。
type ModelDefaultConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
	BaseURL  string `mapstructure:"base_url"`
	APIKey   string `mapstructure:"api_key"`
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
	v.SetDefault("server.write_timeout", 30*time.Second)

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

	v.SetDefault("model.default.provider", "openai")
	v.SetDefault("model.default.model", "gpt-4o-mini")
	v.SetDefault("model.default.base_url", "https://api.openai.com/v1")
	v.SetDefault("model.default.api_key", "")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

	v.SetDefault("storage.local_dir", "./data/uploads")

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
