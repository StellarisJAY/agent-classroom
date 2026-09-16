package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/middleware"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// AuthHandler 认证相关 HTTP 处理器。
type AuthHandler struct {
	svc types.UserService
}

// NewAuthHandler 创建认证处理器。
func NewAuthHandler(svc types.UserService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Register 注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req types.RegisterReq
	if err := bindJSON(c, &req); err != nil {
		Error(c, err)
		return
	}
	info, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, info)
}

// Login 登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req types.LoginReq
	if err := bindJSON(c, &req); err != nil {
		Error(c, err)
		return
	}
	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, resp)
}

// Me 返回当前登录用户
func (h *AuthHandler) Me(c *gin.Context) {
	userID, err := middleware.UserID(c)
	if err != nil {
		Error(c, types.ErrUnauthorized)
		return
	}
	info, err := h.svc.GetMe(c.Request.Context(), userID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, info)
}
