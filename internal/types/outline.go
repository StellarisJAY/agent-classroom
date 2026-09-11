package types

import (
	"context"
	"time"

	"gorm.io/datatypes"
)

// ---- 状态枚举 ----

// OutlineStatus 大纲状态。
const (
	// OutlineStatusDraft 已生成、待用户确认
	OutlineStatusDraft = "draft"
	// OutlineStatusConfirmed 大纲已确认（下一步据此生成 section）
	OutlineStatusConfirmed = "confirmed"
)

// 环节类型（与 section.type 一致；大纲阶段仅用于约束 LLM 输出）。
// demo 拆为三种独立类型，各自维护生成流程、提示词与代码模板（详见 docs/数据库设计.md）。
const (
	SectionTypeSlide        = "slide"
	SectionTypeQuiz         = "quiz"
	SectionTypeDemo3D       = "demo_3d"
	SectionTypeDemoFunction = "demo_function"
	SectionTypeDemoBasic    = "demo_basic"
)

// ---- 实体 ----

// Outline 大纲实体，对应 outline 表。Content 保存原始大纲 JSON（环节列表）。
// Version 为当前生效版本号；每版同时快照到 outline_history。
type Outline struct {
	ID       ID             `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID ID             `gorm:"type:uuid;not null;index" json:"-"`
	Content  datatypes.JSON `gorm:"type:jsonb;not null" json:"content"`
	Status   string         `gorm:"not null;default:draft" json:"status"`
	Version  int            `gorm:"not null;default:1" json:"version"`
	CreateBy *ID            `gorm:"type:uuid" json:"-"`
	CreateAt time.Time      `gorm:"not null;default:now()" json:"-"`
	UpdateBy *ID            `gorm:"type:uuid" json:"-"`
	UpdateAt time.Time      `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (Outline) TableName() string { return "outline" }

// OutlineHistory 大纲历史版本快照，对应 outline_history 表。
// 每次生成/重新生成都会写入一条；Content 为该版完整环节 JSON。
// Feedback 为触发该版的修改意见（首版为空），Title 为该版的课程标题。
type OutlineHistory struct {
	ID        ID             `gorm:"type:uuid;primaryKey" json:"id"`
	OutlineID ID             `gorm:"type:uuid;not null;index" json:"-"`
	Version   int            `gorm:"not null" json:"version"`
	Title     string         `gorm:"not null;default:''" json:"title"`
	Content   datatypes.JSON `gorm:"type:jsonb;not null" json:"content"`
	Feedback  string         `gorm:"not null;default:''" json:"feedback"`
	CreateAt  time.Time      `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (OutlineHistory) TableName() string { return "outline_history" }

// ---- 结构 ----

// OutlineSection 单个大纲环节（环节标题 + 形式 + 知识点 + 内容描述）。
type OutlineSection struct {
	Title           string   `json:"title"`
	Type            string   `json:"type"` // slide | quiz | demo_3d | demo_function | demo_basic
	KnowledgePoints []string `json:"knowledge_points"`
	// Description 大纲阶段对该环节将生成内容的详细描述（slide：讲解内容/组织/示例；
	// quiz：题型与题量、考察方式；demo：演示/交互目标）。确认大纲后作为内容生成的固化要求。
	Description string `json:"description,omitempty"`
}

// OutlineContent 大纲 content 的 jsonb 结构（{"sections":[…] }）。
type OutlineContent struct {
	Sections []OutlineSection `json:"sections"`
}

// OutlineLLMResult LLM 非流式返回的大纲（含自动生成的标题）。
type OutlineLLMResult struct {
	Title    string           `json:"title"`
	Sections []OutlineSection `json:"sections"`
}

// OutlineResult 生成成功后返回给 handler 的结构。
type OutlineResult struct {
	Title    string           `json:"title"`
	Sections []OutlineSection `json:"sections"`
}

// OutlineView 返回给前端的已保存大纲视图。
type OutlineView struct {
	Status   string           `json:"status"`
	Version  int              `json:"version"`
	Sections []OutlineSection `json:"sections"`
}

// OutlineTaskView 大纲生成任务状态视图（供前端轮询）。
// Status 取值：done（大纲已入库）| generating | error | idle（未开始或服务重启丢失）。
type OutlineTaskView struct {
	Status  string       `json:"status"`
	Message string       `json:"message,omitempty"`
	Outline *OutlineView `json:"outline,omitempty"`
}

// OutlineVersionView 大纲历史版本列表项（不含 content，减小体积）。
type OutlineVersionView struct {
	Version   int       `json:"version"`
	Title     string    `json:"title"`
	Feedback  string    `json:"feedback"`
	Current   bool      `json:"current"`
	CreatedAt time.Time `json:"created_at"`
}

// RegenerateOutlineReq 重新生成大纲请求体。Feedback 为修改意见，可为空。
type RegenerateOutlineReq struct {
	Feedback string `json:"feedback"`
}

// RevertOutlineReq 回退大纲到指定版本请求。
type RevertOutlineReq struct {
	Version int `json:"version" binding:"required,min=1"`
}

// ---- 业务错误 ----

var (
	// ErrOutlineNotFound 大纲不存在
	ErrOutlineNotFound = NewError(CodeNotFound, "大纲不存在")
	// ErrOutlineVersionNotFound 指定的大纲版本不存在
	ErrOutlineVersionNotFound = NewError(CodeNotFound, "大纲版本不存在")
	// ErrOutlineGenerating 大纲正在生成中，不能重复触发
	ErrOutlineGenerating = NewError(CodeBadRequest, "大纲正在生成中，请稍候")
)

// ---- 接口 ----

// OutlineRepo 大纲数据访问接口。
type OutlineRepo interface {
	// Create 插入新大纲（ID/时间戳由实现填充）。
	Create(ctx context.Context, o *Outline) error
	// UpdateContentStatus 覆盖指定课程的大纲 content 与 status。
	UpdateContentStatus(ctx context.Context, courseID ID, content datatypes.JSON, status string, by *ID) error
	// UpdateContentVersion 覆盖指定课程的大纲 content 与当前版本号（状态保持 draft）。
	UpdateContentVersion(ctx context.Context, courseID ID, content datatypes.JSON, version int, by *ID) error
	// GetByCourse 返回某课程大纲，未找到返回 ErrNotFound。
	GetByCourse(ctx context.Context, courseID ID) (*Outline, error)
	// DeleteByCourse 删除某课程大纲。
	DeleteByCourse(ctx context.Context, courseID ID) error
}

// OutlineHistoryRepo 大纲历史版本数据访问接口。
type OutlineHistoryRepo interface {
	// Create 插入一条版本快照（ID/时间戳由实现填充）。
	Create(ctx context.Context, h *OutlineHistory) error
	// ListByOutline 返回某大纲全部版本快照，按版本号倒序。
	ListByOutline(ctx context.Context, outlineID ID) ([]OutlineHistory, error)
	// GetByVersion 返回指定版本快照，未找到返回 ErrNotFound。
	GetByVersion(ctx context.Context, outlineID ID, version int) (*OutlineHistory, error)
	// Prune 仅保留版本号最大的 keep 条，删除其余更旧的快照。
	Prune(ctx context.Context, outlineID ID, keep int) error
}
