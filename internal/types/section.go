package types

import (
	"context"
	"time"

	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// ---- 环节生成状态 ----

// SectionStatus 环节内容生成状态。
const (
	// SectionStatusPending 待生成
	SectionStatusPending = "pending"
	// SectionStatusGenerating 生成中
	SectionStatusGenerating = "generating"
	// SectionStatusDone 已生成完成
	SectionStatusDone = "done"
)

// ---- 实体 ----

// Section 环节实体，对应 section 表。由确认后的大纲逐条物化而来。
// Content / Steps 存储按 type 区分的生成产物（详见 docs/slide数据结构.md 与 docs/数据库设计.md）。
type Section struct {
	ID              ID             `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID        ID             `gorm:"type:uuid;not null;index" json:"-"`
	Position        int            `gorm:"not null" json:"position"`
	Type            string         `gorm:"type:section_type;not null" json:"type"`
	Title           string         `gorm:"not null;default:''" json:"title"`
	KnowledgePoints datatypes.JSON `gorm:"type:jsonb;not null" json:"-"`
	Prompt          *string        `gorm:"type:text" json:"prompt,omitempty"`
	Status          string         `gorm:"type:section_status;not null;default:pending" json:"status"`
	Content         datatypes.JSON `gorm:"type:jsonb" json:"content,omitempty"`
	Steps           datatypes.JSON `gorm:"type:jsonb" json:"steps,omitempty"`
	CreateBy        *ID            `gorm:"type:uuid" json:"-"`
	CreateAt        time.Time      `gorm:"not null;default:now()" json:"-"`
	UpdateBy        *ID            `gorm:"type:uuid" json:"-"`
	UpdateAt        time.Time      `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (Section) TableName() string { return "section" }

// ---- DTO ----

// ConfirmOutlineReq 确认大纲请求体。Sections 为前端最终展示的有序环节列表
// （允许含调序/删除后的结果），后端据此覆盖大纲并物化 section。
type ConfirmOutlineReq struct {
	Sections []OutlineSection `json:"sections" binding:"required,min=1,dive"`
}

// SectionProgress 单个环节的生成进度（供确认返回与 SSE 进度展示）。
type SectionProgress struct {
	ID              ID       `json:"id"`
	Position        int      `json:"position"`
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	Status          string   `json:"status"`
	KnowledgePoints []string `json:"knowledge_points"`
}

// ProgressEvent 内容生成过程中的进度事件（经 SSE 推送）。
// Type 取值：snapshot（全量进度，恢复用）| section（单环节状态变更）| course（课程终态）| error。
type ProgressEvent struct {
	Type     string            `json:"type"`
	Sections []SectionProgress `json:"sections,omitempty"`
	Index    int               `json:"index,omitempty"`
	Section  SectionProgress   `json:"section,omitempty"`
	Status   string            `json:"status,omitempty"`
	Message  string            `json:"message,omitempty"`
}

// GenerationContext 传给环节内容生成器的上下文，支撑后续「连贯性」等需求。
// 框架阶段 DocsText / Client / Thinking 由真实生成器实现时接入，占位暂为空。
type GenerationContext struct {
	// Course 所属课程
	Course *Course
	// OutlineSections 全量已确认的有序大纲环节（连贯性参考，如「上一节我们学习了…」）
	OutlineSections []OutlineSection
	// Done 已生成的环节（供衔接上下文）
	Done []Section
	// DocsText 参考文档提取文本摘要（真实生成器接入时填充）
	DocsText string
	// Client 本次生成使用的 LLM 客户端
	Client model.LLMClient
	// Thinking 模型思考限制
	Thinking string
}

// ---- 业务错误 ----

var (
	// ErrOutlineNotConfirmed 尚未确认大纲，不能生成内容
	ErrOutlineNotConfirmed = NewError(CodeBadRequest, "请先生成并确认大纲")
	// ErrOutlineAlreadyConfirmed 大纲已确认或内容正在生成，不能重复确认
	ErrOutlineAlreadyConfirmed = NewError(CodeBadRequest, "大纲已确认，不能重复确认")
	// ErrContentGenerate 环节内容生成失败
	ErrContentGenerate = NewError(CodeInternalError, "课程内容生成失败")
)

// ---- 接口 ----

// SectionRepo 环节数据访问接口。
type SectionRepo interface {
	// CreateBulk 批量插入环节（ID 与时间戳由调用方/实现填充）。
	CreateBulk(ctx context.Context, sections []Section) error
	// ListByCourse 返回某课程全部环节，按 position 升序。
	ListByCourse(ctx context.Context, courseID ID) ([]Section, error)
	// UpdateStatus 更新环节生成状态。
	UpdateStatus(ctx context.Context, id ID, status string) error
	// UpdateContentSteps 更新环节 content 与 steps 产物。
	UpdateContentSteps(ctx context.Context, id ID, content, steps datatypes.JSON) error
}

// SectionContentGenerator 单个环节的内容生成器。产物写入 section.Content / Steps，
// 由生成服务统一负责状态迁移与持久化。
type SectionContentGenerator interface {
	Generate(ctx context.Context, section *Section, genCtx *GenerationContext) error
}

// SectionService 环节内容生成业务接口。
type SectionService interface {
	// ConfirmOutline 确认大纲并物化环节，随后立即启动后台串行生成，返回物化后的环节进度。
	ConfirmOutline(ctx context.Context, userID, courseID ID, req *ConfirmOutlineReq) ([]SectionProgress, error)
	// ListProgress 返回某课程全部环节当前进度。
	ListProgress(ctx context.Context, userID, courseID ID) ([]SectionProgress, error)
	// StreamGeneration 订阅某课程内容生成进度：先回放快照，再实时转发，直到完成/出错/上下文取消。
	// emit 返回 error 时中止订阅。
	StreamGeneration(ctx context.Context, userID, courseID ID, emit func(ProgressEvent) error) error
	// GetLearnDetail 返回课程学习详情：课程摘要 + 有序环节（slide 含 content/steps 产物透传）。
	// 仅课程 owner 可访问；进度本期固定 unstarted（学习进度上报另行实现）。
	GetLearnDetail(ctx context.Context, userID, courseID ID) (*CourseLearnDetail, error)
}
