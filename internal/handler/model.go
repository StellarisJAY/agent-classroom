package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// ModelHandler 平台全局可用模型处理器。
type ModelHandler struct {
	svc types.ModelConfigService
}

// NewModelHandler 创建平台模型处理器。
func NewModelHandler(svc types.ModelConfigService) *ModelHandler {
	return &ModelHandler{svc: svc}
}

// List 返回配置文件中的平台全局可选模型清单（不含密钥）。
func (h *ModelHandler) List(c *gin.Context) {
	OK(c, h.svc.Options(c.Request.Context()))
}
