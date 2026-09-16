package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// discussionSSESink 把 SSE 事件写出包装为 types.DiscussionSink（讨论模块特有的适配器）。
type discussionSSESink struct {
	w *sseWriter
}

func (s discussionSSESink) Text(delta string) error {
	return s.w.write(sseEvent{Type: "text", Delta: delta})
}

func (s discussionSSESink) Action(name string, arguments string) error {
	args := json.RawMessage(arguments)
	if !json.Valid(args) {
		args = nil
	}
	return s.w.write(sseEvent{Type: "action", Name: name, Args: args})
}

func (s discussionSSESink) End() error {
	return s.w.write(sseEvent{Type: "end"})
}

func (s discussionSSESink) Error(msg string) {
	_ = s.w.write(sseEvent{Type: "error", Msg: msg})
}

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
	_ = h.svc.Ask(c.Request.Context(), userID, courseID, req, discussionSSESink{w: w})
}

// ListConversations 拉取课程下的会话列表（按最近活跃倒序）。
func (h *DiscussionHandler) ListConversations(c *gin.Context) {
	userID, ok := currentUser(c)
	if !ok {
		return
	}
	courseID, err := pathID(c)
	if err != nil {
		Error(c, err)
		return
	}
	items, err := h.svc.ListConversations(c.Request.Context(), userID, courseID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, items)
}

// ListConversation 拉取一处会话的问答历史（结构化消息，前端按末态规则重放动作）。
// 查询参数 conversation_id 指定会话；未传时返回最近活跃会话的消息。
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
	var convID types.ID
	if raw := c.Query("conversation_id"); raw != "" {
		id, parseErr := types.ParseID(raw)
		if parseErr != nil {
			Error(c, types.ErrInvalidRequest)
			return
		}
		convID = id
	}
	msgs, err := h.svc.ListConversation(c.Request.Context(), userID, courseID, convID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, msgs)
}
