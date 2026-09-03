package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// CourseHandler 课程相关 HTTP 处理器。
type CourseHandler struct {
	svc types.CourseService
}

// NewCourseHandler 创建课程处理器。
func NewCourseHandler(svc types.CourseService) *CourseHandler {
	return &CourseHandler{svc: svc}
}

// List 查询当前用户课程列表（支持筛选 + 分页）。
func (h *CourseHandler) List(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	var req types.CourseListReq
	if err := bindQuery(c, &req); err != nil {
		Error(c, err)
		return
	}
	resp, err := h.svc.List(c.Request.Context(), userID, &req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, resp)
}
