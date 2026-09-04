package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// SectionHandler 课程环节内容生成相关 HTTP 处理器。
type SectionHandler struct {
	svc types.SectionService
}

// NewSectionHandler 创建环节处理器。
func NewSectionHandler(svc types.SectionService) *SectionHandler {
	return &SectionHandler{svc: svc}
}

// Confirm 确认大纲：接收前端最终有序环节列表，覆盖大纲并物化，随后后台串行生成。
func (h *SectionHandler) Confirm(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	var req types.ConfirmOutlineReq
	if err := bindJSON(c, &req); err != nil {
		Error(c, err)
		return
	}
	sections, err := h.svc.ConfirmOutline(c.Request.Context(), userID, courseID, &req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, sections)
}

// List 查询某课程全部环节当前生成进度。
func (h *SectionHandler) List(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	sections, err := h.svc.ListProgress(c.Request.Context(), userID, courseID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, sections)
}

// Learn 查询课程学习详情（课程摘要 + 有序环节，slide 含 content/steps 产物）。
func (h *SectionHandler) Learn(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	detail, err := h.svc.GetLearnDetail(c.Request.Context(), userID, courseID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, detail)
}

// Generate 订阅某课程内容生成进度（SSE）。事件：snapshot / section / course / done / error。
func (h *SectionHandler) Generate(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}

	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flush(w)

	if err := util.WriteSSEEvent(w, "start", "{}"); err != nil {
		return
	}
	flush(w)

	err = h.svc.StreamGeneration(c.Request.Context(), userID, courseID, func(ev types.ProgressEvent) error {
		data, merr := json.Marshal(ev)
		if merr != nil {
			return merr
		}
		return util.WriteSSEEvent(w, ev.Type, string(data))
	})
	if err != nil {
		_ = util.WriteSSEError(w, errSSEMessage(err))
		flush(w)
		return
	}
	_ = util.WriteSSEDone(w)
	flush(w)
}
