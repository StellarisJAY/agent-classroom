package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/middleware"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// ModelConfigHandler 用户模型配置相关 HTTP 处理器。
type ModelConfigHandler struct {
	svc types.ModelConfigService
}

// NewModelConfigHandler 创建模型配置处理器。
func NewModelConfigHandler(svc types.ModelConfigService) *ModelConfigHandler {
	return &ModelConfigHandler{svc: svc}
}

// List 查询当前用户全部模型配置
func (h *ModelConfigHandler) List(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	infos, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, infos)
}

// Create 新增模型配置
func (h *ModelConfigHandler) Create(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	var req types.CreateModelConfigReq
	if err := bindJSON(c, &req); err != nil {
		Error(c, err)
		return
	}
	info, err := h.svc.Create(c.Request.Context(), userID, req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, info)
}

// Update 编辑模型配置
func (h *ModelConfigHandler) Update(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	id, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	var req types.UpdateModelConfigReq
	if err := bindJSON(c, &req); err != nil {
		Error(c, err)
		return
	}
	info, err := h.svc.Update(c.Request.Context(), userID, id, req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, info)
}

// Delete 删除模型配置
func (h *ModelConfigHandler) Delete(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	id, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), userID, id); err != nil {
		Error(c, err)
		return
	}
	OKNoData(c)
}

// SetDefault 设为默认模型配置
func (h *ModelConfigHandler) SetDefault(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	id, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	if err := h.svc.SetDefault(c.Request.Context(), userID, id); err != nil {
		Error(c, err)
		return
	}
	OKNoData(c)
}

// currentUser 从鉴权上下文取当前用户 ID；失败已写响应，返回 false。
func currentUser(c *gin.Context) (types.ID, bool) {
	userID, err := middleware.UserID(c)
	if err != nil {
		Error(c, types.ErrUnauthorized)
		return types.NilID, false
	}
	return userID, true
}
