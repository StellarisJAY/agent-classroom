package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/application/repo"
	"github.com/StellarisJAY/agent-classroom/internal/application/service"
	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/handler"
	"github.com/StellarisJAY/agent-classroom/internal/router"
)

// App 持有已装配的运行时依赖
type App struct {
	cfg    *config.Config
	db     *gorm.DB
	engine *gin.Engine
	logger *slog.Logger
}

// NewApp 装配全部依赖
func NewApp(cfg *config.Config) (*App, error) {
	logger := newLogger(cfg.Log.Level, cfg.Log.Format)
	slog.SetDefault(logger)

	gin.SetMode(cfg.Server.Mode)

	db, err := repo.NewDB(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	// 迁移工具落地前的占位 AutoMigrate
	if err := repo.Migrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	userRepo := repo.NewUserRepo(db)
	userSvc := service.NewUserService(userRepo, cfg)
	authHandler := handler.NewAuthHandler(userSvc)

	e := gin.New()
	router.Register(e, cfg, logger, authHandler)

	return &App{cfg: cfg, db: db, engine: e, logger: logger}, nil
}

// Run 启动 HTTP 服务，监听系统信号优雅关闭
func (a *App) Run() error {
	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", a.cfg.Server.Port),
		Handler:        a.engine,
		ReadTimeout:    a.cfg.Server.ReadTimeout,
		WriteTimeout:   a.cfg.Server.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("server starting", "addr", srv.Addr, "mode", a.cfg.Server.Mode)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	a.logger.Info("server shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}

// Close 释放资源（数据库连接等）
func (a *App) Close() error {
	if a.db == nil {
		return nil
	}
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
