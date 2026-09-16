package router

import (
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/handler"
	"github.com/StellarisJAY/agent-classroom/internal/middleware"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// Register 挂载全局中间件并注册路由分组。
func Register(e *gin.Engine, cfg *config.Config, logger *slog.Logger, auth *handler.AuthHandler, modelConfig *handler.ModelConfigHandler, course *handler.CourseHandler, section *handler.SectionHandler, discussion *handler.DiscussionHandler, storage types.Storage) {
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

	// 参考文档静态访问（经存储抽象，避免绑定具体后端）
	e.GET("/uploads/*path", uploadsHandler(storage))

	// API 根分组
	api := e.Group("/api")
	registerAPI(api, cfg, auth, modelConfig, course, section, discussion)
}

// registerAPI 集中注册所有业务路由分组。
func registerAPI(api *gin.RouterGroup, cfg *config.Config, auth *handler.AuthHandler, modelConfig *handler.ModelConfigHandler, course *handler.CourseHandler, section *handler.SectionHandler, discussion *handler.DiscussionHandler) {
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
		courseGroup.POST("", course.Create)
		courseGroup.GET("/:id/documents", course.ListDocuments)
		courseGroup.GET("/:id/outline", course.GetOutline)
		courseGroup.POST("/:id/outline/regenerate", course.RegenerateOutline)
		courseGroup.GET("/:id/outline/task", course.OutlineTask)
		courseGroup.GET("/:id/outline/versions", course.ListOutlineVersions)
		courseGroup.POST("/:id/outline/revert", course.RevertOutline)
		courseGroup.POST("/:id/outline/confirm", section.Confirm)
		courseGroup.GET("/:id/sections", section.List)
		courseGroup.POST("/:id/generate/resume", section.Generate)
		courseGroup.GET("/:id/learn", section.Learn)
		// 讨论模式：提问（SSE 单流 agent loop）、会话列表与问答历史。
		courseGroup.POST("/:id/questions", discussion.AskQuestion)
		courseGroup.GET("/:id/conversations", discussion.ListConversations)
		courseGroup.GET("/:id/conversation", discussion.ListConversation)
	}
}

// uploadsHandler 从存储抽象读取文件并原样返回。
func uploadsHandler(storage types.Storage) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		rc, err := storage.Get(c.Request.Context(), "/uploads"+path)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer rc.Close()

		ct := mime.TypeByExtension(filepath.Ext(path))
		if ct == "" {
			ct = "application/octet-stream"
		}
		c.Header("Content-Type", ct)
		_, _ = io.Copy(c.Writer, rc)
	}
}
