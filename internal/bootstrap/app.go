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
	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/model/extractor"
	"github.com/StellarisJAY/agent-classroom/internal/model/image"
	"github.com/StellarisJAY/agent-classroom/internal/model/llm"
	"github.com/StellarisJAY/agent-classroom/internal/router"
	"github.com/StellarisJAY/agent-classroom/internal/storage"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// App 持有已装配的运行时依赖
type App struct {
	cfg           *config.Config
	db            *gorm.DB
	engine        *gin.Engine
	logger        *slog.Logger
	modelRegistry *model.Registry
}

// newDocExtractor 按配置创建文档提取器：mineru 仅外部；chain 外部优先本地兜底；local 仅本地。
// chain 下未配置 minerU 端点/token 时链路退化为仅本地。
func newDocExtractor(cfg *config.Config) extractor.Extractor {
	local := extractor.NewLocal()
	mineruCfg := extractor.MineruConfig{
		BaseURL:    cfg.Extractor.Mineru.BaseURL,
		AdminToken: cfg.Extractor.Mineru.AdminToken,
		Timeout:    cfg.Extractor.Mineru.Timeout,
	}
	switch cfg.Extractor.Provider {
	case "local":
		return local
	case "mineru":
		return extractor.NewMineru(mineruCfg)
	default:
		var mineruClient *extractor.Mineru
		if mineruCfg.BaseURL != "" || mineruCfg.AdminToken != "" {
			mineruClient = extractor.NewMineru(mineruCfg)
		}
		return extractor.NewChain(local, mineruClient)
	}
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
	cipher, err := util.NewGCMCipher([]byte(cfg.Crypto.EncryptionKey))
	if err != nil {
		return nil, fmt.Errorf("init crypto cipher: %w", err)
	}

	store := repo.NewStore(db)

	userRepo := repo.NewUserRepo(db)
	userSvc := service.NewUserService(userRepo, cfg)
	authHandler := handler.NewAuthHandler(userSvc)

	modelConfigRepo := repo.NewModelConfigRepo(db)
	modelConfigSvc := service.NewModelConfigService(modelConfigRepo, store, cipher, cfg)
	// 用户模型配置页面/接口暂时屏蔽，仅保留全局模型清单接口。
	// modelConfigHandler := handler.NewModelConfigHandler(modelConfigSvc)
	modelHandler := handler.NewModelHandler(modelConfigSvc)

	// 模型适配层注册表：默认回退 OpenAI 兼容实现，覆盖 openai/deepseek/bailian 等。
	modelRegistry := model.NewRegistry()
	modelRegistry.SetDefaultLLMFactory(llm.NewOpenAICompatible)
	modelRegistry.SetDefaultImageFactory(image.NewOpenAICompatible)
	// 阿里云百炼走专属多模态协议，单独适配。
	modelRegistry.RegisterImage(llm.ProviderBailian, image.NewBailian)

	var objStorage types.Storage
	switch cfg.Storage.Type {
	case "minio":
		objStorage, err = storage.NewMinio(cfg.Storage.Minio)
		if err != nil {
			return nil, fmt.Errorf("init minio storage: %w", err)
		}
	default:
		objStorage, err = storage.NewLocal(cfg.Storage.LocalDir)
		if err != nil {
			return nil, fmt.Errorf("init local storage: %w", err)
		}
	}

	// 参考文档提取链：pdf/docx 优先 minerU（未配置则退化），失败回退本地
	documentRepo := repo.NewDocumentRepo(db)
	docExtractor := newDocExtractor(cfg)
	docLoader := service.NewDocLoader(documentRepo, objStorage, docExtractor, service.DocBudget{
		MaxTokens:     cfg.Extractor.MaxDocTokens,
		CharsPerToken: cfg.Extractor.CharsPerToken,
	}, cfg.Extractor.ExtractTimeout)

	courseRepo := repo.NewCourseRepo(db)
	outlineRepo := repo.NewOutlineRepo(db)
	outlineHistoryRepo := repo.NewOutlineHistoryRepo(db)
	sectionRepo := repo.NewSectionRepo(db)
	questionRepo := repo.NewQuestionRepo(db)
	courseSvc := service.NewCourseService(courseRepo, outlineRepo, outlineHistoryRepo, documentRepo, store, objStorage, modelConfigSvc, modelRegistry, docLoader, cfg)
	courseHandler := handler.NewCourseHandler(courseSvc)
	sectionSvc := service.NewSectionService(courseRepo, outlineRepo, sectionRepo, questionRepo, store, documentRepo, objStorage, modelConfigSvc, modelRegistry, docLoader, cfg)
	sectionHandler := handler.NewSectionHandler(sectionSvc)

	// 讨论模式：课程级问答会话（agent loop 驱动）
	conversationRepo := repo.NewConversationRepo(db)
	discussionSvc := service.NewDiscussionService(courseRepo, sectionRepo, questionRepo, conversationRepo, modelConfigSvc, modelRegistry)
	discussionHandler := handler.NewDiscussionHandler(discussionSvc)

	e := gin.New()
	router.Register(e, cfg, logger, authHandler, modelHandler, courseHandler, sectionHandler, discussionHandler, objStorage)

	return &App{cfg: cfg, db: db, engine: e, logger: logger, modelRegistry: modelRegistry}, nil
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
