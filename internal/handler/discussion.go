package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// DiscussionHandler 讨论模式：提问（SSE）与问答历史。
type DiscussionHandler struct {
	svc types.DiscussionService
}

// NewDiscussionHandler 创建讨论模式 handler。
func NewDiscussionHandler(svc types.DiscussionService) *DiscussionHandler {
	return &DiscussionHandler{svc: svc}
}

// AskQuestion 提问并建立 SSE 流：服务端在流内完成整个 agent loop，
// 事件为 {type:"text",delta} / {type:"action",name,args} / {type:"end"} / {type:"error",msg}。
// 流建立前的业务校验失败走统一 JSON 响应；流建立后的失败发 error 事件后关闭。
func (h *DiscussionHandler) AskQuestion(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	var req types.AskQuestionReq
	if err := bindJSON(c, &req); err != nil {
		Error(c, err)
		return
	}

	w, err := openSSE(c)
	if err != nil {
		Error(c, err)
		return
	}
	defer w.Close()

	// Ask 内部 loop 绑定请求 context：断连时 loop 随 ctx 取消，已落库消息保留。
	_ = h.svc.Ask(c.Request.Context(), userID, courseID, &req, sseSink{w: w})
}

// ListConversation 拉取课程级问答历史（结构化消息，前端按末态规则重放动作）。
func (h *DiscussionHandler) ListConversation(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	msgs, err := h.svc.ListConversation(c.Request.Context(), userID, courseID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, msgs)
}
