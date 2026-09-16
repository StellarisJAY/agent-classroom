package types

import (
	"context"
	"encoding/json"
	"time"
)

// 讨论模式：课程级问答会话（Conversation）与消息（Message），
// 对齐 docs/讨论模式方案.md 与 docs/数据库设计.md 3.8 / 3.9。

// 消息角色。
const (
	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
	MessageRoleTool      = "tool"
)

// MessageContent 消息 content（jsonb）按 role 的结构约定：
//
//	user      → {"text": "..."}
//	assistant → {"text": "..."}（纯文本轮）或 {"tool_calls": [{ "id", "name", "arguments" }]}
//	tool      → {"tool_call_id": "...", "name": "...", "result": "..."}
//
// service 组装 LLM 上下文时严格还原 assistant(tool_call) ↔ tool 成对序列。
type MessageContent struct {
	Text      string       `json:"text,omitempty"`
	ToolCalls []ToolCallEx `json:"tool_calls,omitempty"`
	// ToolCallID / Name / Result 供 tool 角色消息使用。
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Result     string `json:"result,omitempty"`
}

// ToolCallEx 落库的 assistant 工具调用记录。
type ToolCallEx struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Conversation 用户课程下的问答会话实体（每用户每课程可建多条）。
// Title 为会话标题：首问时按用户提问前 10 个字符写入（其余提问不更新）。
type Conversation struct {
	ID       ID        `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID ID        `gorm:"type:uuid;not null" json:"course_id"`
	UserID   ID        `gorm:"type:uuid;not null" json:"user_id"`
	Title    string    `gorm:"type:varchar(16);not null;default:''" json:"title"`
	CreateBy *ID       `gorm:"type:uuid" json:"-"`
	CreateAt time.Time `gorm:"not null;default:now()" json:"create_at"`
	UpdateAt time.Time `gorm:"not null;default:now()" json:"update_at"`
}

func (Conversation) TableName() string { return "conversation" }

// Message 单条消息。SectionID 仅 user 消息有意义（提问来源环节），
// 后续项按 数据库设计.md 3.9（section 删除时 SET NULL）。
type Message struct {
	ID             ID              `gorm:"type:uuid;primaryKey" json:"id"`
	ConversationID ID              `gorm:"type:uuid;not null" json:"-"`
	Role           string          `gorm:"type:message_role;not null" json:"role"`
	Content        json.RawMessage `gorm:"type:jsonb;not null" json:"content"`
	SectionID      *ID             `gorm:"type:uuid" json:"section_id,omitempty"`
	CreateAt       time.Time       `gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

func (Message) TableName() string { return "message" }

// ConversationRepo 问答会话数据访问接口。
type ConversationRepo interface {
	// Create 新建一条空会话（title 为空，首问时写入标题）。
	Create(ctx context.Context, courseID, userID ID) (*Conversation, error)
	// GetByID 按会话 ID 取会话（校验属主：user_id 必须匹配，不匹配按 not found 处理）。
	GetByID(ctx context.Context, userID, id ID) (*Conversation, error)
	// ListByCourse 返回某用户某课程的全部会话，按 update_at 降序（最近活跃在前）。
	ListByCourse(ctx context.Context, courseID, userID ID) ([]Conversation, error)
	// UpdateTitle 更新会话标题并刷新 update_at（同一事务保证一致）。
	UpdateTitle(ctx context.Context, id ID, title string) error
	// ListMessages 按时间升序返回会话全部消息。
	ListMessages(ctx context.Context, conversationID ID) ([]Message, error)
	// Append 追加一条消息并刷新会话 update_at。
	Append(ctx context.Context, conversationID ID, role string, content json.RawMessage, sectionID *ID) error
}

// ---- DTO ----

// AskQuestionReq 讨论模式提问请求体。
type AskQuestionReq struct {
	// Question 提问正文。
	Question string `json:"question" binding:"required,max=2000"`
	// SectionID 提问来源环节（可空：脱离环节的全局提问）。
	SectionID *ID `json:"section_id,omitempty"`
	// StepIndex 提问时前端所处的讲解步骤下标（前端瞬时态，仅用于上下文标注）。
	StepIndex *int `json:"step_index,omitempty"`
	// ConversationID 目标会话（可空：不传则隐式新建一条会话，即"新对话"）。
	ConversationID *ID `json:"conversation_id,omitempty"`
}

// ConversationItemResp 会话列表项（GET /courses/:id/conversations）。
type ConversationItemResp struct {
	ID       ID        `json:"id"`
	Title    string    `json:"title"`
	UpdateAt time.Time `json:"update_at"`
}

// ConversationMessageResp 问答历史消息项（GET /courses/:id/conversation）。
type ConversationMessageResp struct {
	ID        ID              `json:"id"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	SectionID *ID             `json:"section_id,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// ---- 业务错误 ----

var (
	// ErrDiscussionGenerating 课程内容尚未全部生成完成，禁止进入讨论
	ErrDiscussionGenerating = NewError(CodeConflict, "课程内容生成中，暂不能提问")
	// ErrDiscussionBusy 同一会话已有提问在处理中
	ErrDiscussionBusy = NewError(CodeConflict, "上一条提问还在回答中，请稍后再试")
	// ErrConversationNotFound 会话不存在或不属于当前用户
	ErrConversationNotFound = NewError(CodeNotFound, "会话不存在")
)

// ---- 接口 ----

// DiscussionSink 讨论模式的事件汇（handler 侧实现为 SSE 写出）。
// Ask 过程中逐个回调；流建立后任何失败经 Error 通告。
type DiscussionSink interface {
	// Text 文本增量
	Text(delta string) error
	// Action 一次工具调用动作（arguments 为模型给出的参数 JSON 原文，透传给前端）
	Action(name string, arguments string) error
	// End 正常收尾
	End() error
	// Error 流式过程中的失败（非致命：断连由 SSE 自行感知）
	Error(msg string)
}

// DiscussionService 讨论模式业务接口。
type DiscussionService interface {
	// Ask 处理一次提问：服务端装配上下文 → 跑 agent loop → 事件经 sink 流出 →
	// 逐条落库。req.ConversationID 为空时隐式新建会话（新对话）。
	// 会话首次提问时以其前 10 个字符作为会话标题。
	// 返回 error 表示 loop 失败（含业务校验类错误与 LLM 失败）。
	Ask(ctx context.Context, userID, courseID ID, req AskQuestionReq, sink DiscussionSink) error
	// ListConversations 返回某课程下当前用户全部会话（按最近活跃倒序）。
	ListConversations(ctx context.Context, userID, courseID ID) ([]ConversationItemResp, error)
	// ListConversation 返回指定会话的问答历史结构化消息（前端按末态规则重放动作）。
	// conversationID 为空时取最近活跃会话（无会话返回空数组）。
	ListConversation(ctx context.Context, userID, courseID, conversationID ID) ([]ConversationMessageResp, error)
}
