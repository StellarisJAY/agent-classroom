package repo

import (
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// testDB 从环境变量构建测试数据库连接；未设置则跳过。
// 用法：TEST_DB_HOST=... TEST_DB_PORT=... TEST_DB_USER=... TEST_DB_PASSWORD=... TEST_DB_NAME=... go test ./...
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	host := envOr("TEST_DB_HOST", "localhost")
	port := envOr("TEST_DB_PORT", "5432")
	user := envOr("TEST_DB_USER", "postgres")
	password := os.Getenv("TEST_DB_PASSWORD")
	dbname := envOr("TEST_DB_NAME", "agent_classroom_test")
	if password == "" {
		t.Skip("TEST_DB_PASSWORD not set; skipping DB tests")
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		host, port, user, password, dbname)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	return db
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
