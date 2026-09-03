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
const (
	SectionTypeSlide = "slide"
	SectionTypeQuiz  = "quiz"
	SectionTypeDemo  = "demo"
)

// ---- 实体 ----

// Outline 大纲实体，对应 outline 表。Content 保存原始大纲 JSON（环节列表）。
type Outline struct {
	ID       ID             `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID ID             `gorm:"type:uuid;not null;index" json:"-"`
	Content  datatypes.JSON `gorm:"type:jsonb;not null" json:"content"`
	Status   string         `gorm:"not null;default:draft" json:"status"`
	CreateBy *ID            `gorm:"type:uuid" json:"-"`
	CreateAt time.Time      `gorm:"not null;default:now()" json:"-"`
	UpdateBy *ID            `gorm:"type:uuid" json:"-"`
	UpdateAt time.Time      `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (Outline) TableName() string { return "outline" }

// ---- 结构 ----

// OutlineSection 单个大纲环节（环节标题 + 形式 + 知识点）。
type OutlineSection struct {
	Title           string   `json:"title"`
	Type            string   `json:"type"` // slide | quiz | demo
	KnowledgePoints []string `json:"knowledge_points"`
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
	Sections []OutlineSection `json:"sections"`
}

// ---- 业务错误 ----

var (
	// ErrOutlineNotFound 大纲不存在
	ErrOutlineNotFound = NewError(CodeNotFound, "大纲不存在")
)

// ---- 接口 ----

// OutlineRepo 大纲数据访问接口。
type OutlineRepo interface {
	// Create 插入新大纲（ID/时间戳由实现填充）。
	Create(ctx context.Context, o *Outline) error
	// UpdateContentStatus 覆盖指定课程的大纲 content 与 status。
	UpdateContentStatus(ctx context.Context, courseID ID, content datatypes.JSON, status string, by *ID) error
	// GetByCourse 返回某课程大纲，未找到返回 ErrNotFound。
	GetByCourse(ctx context.Context, courseID ID) (*Outline, error)
	// DeleteByCourse 删除某课程大纲。
	DeleteByCourse(ctx context.Context, courseID ID) error
}
