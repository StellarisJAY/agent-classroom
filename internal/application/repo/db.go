package repo

import (
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/StellarisJAY/agent-classroom/internal/config"
)

// NewDB 初始化 GORM + postgres 连接池
func NewDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	}
	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	slog.Info("database connected",
		"host", cfg.Host, "port", cfg.Port, "dbname", cfg.DBName,
		"max_open", cfg.MaxOpenConns, "max_idle", cfg.MaxIdleConns,
	)
	return db, nil
}

// Ping 确保连接可用
func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
