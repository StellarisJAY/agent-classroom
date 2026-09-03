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
	Log      LogConfig
}

type ServerConfig struct {
	Port         int
	Mode         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	Timezone        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type JWTConfig struct {
	Secret      string
	ExpireHours int
}

type CORSConfig struct {
	AllowedOrigins []string
	AllowedHeaders []string
	MaxAge         int
}

type LogConfig struct {
	Level  string
	Format string
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

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

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
