package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/handler"
	"github.com/StellarisJAY/agent-classroom/internal/middleware"
)

// Register 挂载全局中间件并注册路由分组。
func Register(e *gin.Engine, cfg *config.Config, logger *slog.Logger, auth *handler.AuthHandler, modelConfig *handler.ModelConfigHandler, course *handler.CourseHandler) {
	// 全局中间件
	e.Use(
		middleware.Recovery(logger),
		middleware.Logger(logger),
		middleware.CORS(cfg.CORS.AllowedOrigins, cfg.CORS.AllowedHeaders, cfg.CORS.MaxAge),
	)

	// 健康检查（无鉴权）
	e.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API 根分组
	api := e.Group("/api")
	registerAPI(api, cfg, auth, modelConfig, course)
}

// registerAPI 集中注册所有业务路由分组。
func registerAPI(api *gin.RouterGroup, cfg *config.Config, auth *handler.AuthHandler, modelConfig *handler.ModelConfigHandler, course *handler.CourseHandler) {
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", auth.Register)
		authGroup.POST("/login", auth.Login)
		authGroup.GET("/me", middleware.Auth(cfg.JWT.Secret), auth.Me)
	}

	modelConfigGroup := api.Group("/model-configs", middleware.Auth(cfg.JWT.Secret))
	{
		modelConfigGroup.GET("", modelConfig.List)
		modelConfigGroup.POST("", modelConfig.Create)
		modelConfigGroup.PUT("/:id", modelConfig.Update)
		modelConfigGroup.DELETE("/:id", modelConfig.Delete)
		modelConfigGroup.PUT("/:id/default", modelConfig.SetDefault)
	}

	courseGroup := api.Group("/courses", middleware.Auth(cfg.JWT.Secret))
	{
		courseGroup.GET("", course.List)
	}
}
