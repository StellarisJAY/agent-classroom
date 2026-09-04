package types

import (
	"context"
	"time"
)

// ---- 状态枚举 ----

const (
	// CourseStatusDraft 草稿（初始创建后未确认大纲）
	CourseStatusDraft = "draft"
	// CourseStatusOutlineConfirmed 大纲已确认（可生成内容）
	CourseStatusOutlineConfirmed = "outline_confirmed"
	// CourseStatusGenerating 内容生成中
	CourseStatusGenerating = "generating"
	// CourseStatusCompleted 内容已生成完成，可学习
	CourseStatusCompleted = "completed"
)

// ProgressStatus 学习进度（每用户每课程），对应 progress.status
const (
	ProgressStatusUnstarted  = "unstarted"
	ProgressStatusInProgress = "in_progress"
	ProgressStatusCompleted  = "completed"
)

// Scope 课程列表归属筛选
const (
	CourseScopeAll    = "all"
	CourseScopeMine   = "mine"
	CourseScopePublic = "public"
)

// 大纲环节数量档位
const (
	// DefaultOutlineCount 默认 / 最小环节数
	DefaultOutlineCount = 5
	// MinOutlineCount 允许的最小环节数
	MinOutlineCount = 5
	// MaxOutlineCount 允许的最大环节数
	MaxOutlineCount = 30
)

// ---- 实体 ----

// Course 课程实体，对应 course 表。
// Title 由大纲生成时 LLM 产出（创建 draft 时为空）；Prompt 为用户输入的课程内容要求，
// 是大纲生成的唯一提示词依据（参考文档提取文本不入库，仅生成流程临时读取）。
type Course struct {
	ID       ID     `gorm:"type:uuid;primaryKey" json:"id"`
	OwnerID  ID     `gorm:"type:uuid;not null" json:"-"`
	Title    string `gorm:"not null;default:''" json:"title"`
	Prompt   string `gorm:"not null" json:"prompt"`
	Status   string `gorm:"type:course_status;not null;default:draft" json:"status"`
	IsPublic bool   `gorm:"not null;default:false" json:"is_public"`
	// ModelConfigID 该课程使用哪份模型配置；为空则大纲生成时回退默认配置。
	ModelConfigID *ID `gorm:"type:uuid" json:"model_config_id"`
	// Thinking 生成所用模型的思考限制：off / default / max。
	Thinking string `gorm:"not null;default:default" json:"thinking"`
	// OutlineCount 大纲环节数量上限（用户可调）；为空回退默认。
	OutlineCount int       `gorm:"not null;default:5" json:"outline_count"`
	CreateBy     *ID       `gorm:"type:uuid" json:"-"`
	CreateAt     time.Time `gorm:"not null;default:now()" json:"-"`
	UpdateAt     time.Time `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (Course) TableName() string { return "course" }

// Progress 学习进度实体，对应 progress 表（user + course 关联）。
type Progress struct {
	ID       ID        `gorm:"type:uuid;primaryKey" json:"-"`
	CourseID ID        `gorm:"type:uuid;not null" json:"-"`
	UserID   ID        `gorm:"type:uuid;not null" json:"-"`
	Status   string    `gorm:"type:progress_status;not null;default:unstarted" json:"status"`
	CreateAt time.Time `gorm:"not null;default:now()" json:"-"`
	UpdateAt time.Time `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (Progress) TableName() string { return "progress" }

// ---- DTO ----

// CourseListItem 课程列表项（供前端卡片渲染与筛选展示）。
type CourseListItem struct {
	ID       ID     `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	IsPublic bool   `json:"is_public"`
	// Owned 当前用户是否为创建者（owner）
	Owned bool `json:"owned"`
	// Progress 当前用户对该课程的学习进度；无记录默认 unstarted
	Progress  string    `json:"progress"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CourseListReq 课程列表查询参数（GET query）。
type CourseListReq struct {
	// Scope 归属筛选：all | mine | public
	Scope string `form:"scope" binding:"omitempty,oneof=all mine public"`
	// Keyword 标题模糊搜索
	Keyword string `form:"keyword"`
	// Progress 学习状态筛选：unstarted | in_progress | completed
	Progress string `form:"progress" binding:"omitempty,oneof=unstarted in_progress completed"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// CourseListResp 分页响应。
type CourseListResp struct {
	Items    []CourseListItem `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// CourseListFilter 传给 repo 的过滤条件（含分页）。
type CourseListFilter struct {
	Scope    string
	Keyword  string
	Progress string
	Page     int
	PageSize int
	Offset   int
}

// CourseListRow repo 扫描目标：课程行 + 当前用户进度（LEFT JOIN 派生）。
type CourseListRow struct {
	Course
	// ProgressStatus 当前用户进度；无 progress 记录时兜底为 unstarted
	ProgressStatus string
}

// ---- DTO ----

// CreateCourseReq 创建课程请求（Step 0）。由 handler 从 multipart 解析后填充。
type CreateCourseReq struct {
	// Prompt 用户输入的课程内容要求（必填，作为大纲生成提示词）
	Prompt string
	// Files 上传的参考文档（早期仅 .txt/.md，文本提取后不落库）
	Files []UploadedFile
	// ModelConfigID 选择的用户模型配置；为空则生成时用默认配置。
	ModelConfigID *ID
	// Thinking 模型思考限制：off / default / max。
	Thinking string
	// OutlineCount 大纲环节数量上限；0 或缺省由 service 回退默认（5）。
	OutlineCount int
}

// UploadedFile 一个已读入内存的上传文档。Name 为原始文件名（含扩展名），Data 为文件内容。
type UploadedFile struct {
	Name string
	Data []byte
}

// CourseCreateResp 创建课程成功响应。
type CourseCreateResp struct {
	ID     ID     `json:"id"`
	Title  string `json:"title"`
	Prompt string `json:"prompt"`
	Status string `json:"status"`
}

// ---- 业务错误 ----

var (
	// ErrInvalidScope 归属筛选取值不合法
	ErrInvalidScope = NewError(CodeBadRequest, "归属筛选不合法")
	// ErrCourseNotFound 课程不存在或无访问权限
	ErrCourseNotFound = NewError(CodeNotFound, "课程不存在")
	// ErrPromptRequired prompt 不能为空
	ErrPromptRequired = NewError(CodeBadRequest, "请输入课程内容要求")
	// ErrNoModelConfig 无可用模型配置（未设默认且服务端兜底缺失）
	ErrNoModelConfig = NewError(CodeBadRequest, "未配置可用模型，请先在设置中添加模型配置")
	// ErrUnsupportedFile 参考文档格式不支持（早期仅 txt/md）
	ErrUnsupportedFile = NewError(CodeBadRequest, "参考文档仅支持 txt/md 格式")
	// ErrFileTooLarge 文档超过大小上限
	ErrFileTooLarge = NewError(CodeBadRequest, "单个参考文档不能超过 10MB")
	// ErrOutlineFailed 大纲生成失败（上游/解析错误）
	ErrOutlineFailed = NewError(CodeInternalError, "大纲生成失败，请重试")
)

// ---- 接口 ----

// CourseRepo 课程数据访问接口。
type CourseRepo interface {
	// List 按过滤条件分页查询当前用户可见课程，返回行列表与命中总数。
	// 行内含 LEFT JOIN progress 得到的当前用户进度（无记录时 progress_status 为空串）。
	List(ctx context.Context, userID ID, f CourseListFilter) ([]CourseListRow, int64, error)
	// Create 插入新课程（ID/时间戳由实现填充）。
	Create(ctx context.Context, c *Course) error
	// GetByID 按主键 + owner_id 查询，未找到返回 ErrNotFound。
	GetByID(ctx context.Context, ownerID, id ID) (*Course, error)
	// UpdateTitle 更新课程标题（大纲生成后回填）。
	UpdateTitle(ctx context.Context, id ID, title string) error
	// UpdateStatus 更新课程状态。
	UpdateStatus(ctx context.Context, id ID, status string) error
}

// CourseService 课程业务接口。
type CourseService interface {
	// List 分页返回当前用户课程列表；req 为 nil 或缺省字段时取默认（all / 第 1 页 / 每页 20）。
	List(ctx context.Context, userID ID, req *CourseListReq) (*CourseListResp, error)
	// Create 创建草稿课程并保存参考文档元数据（提取的文本不入库）。
	Create(ctx context.Context, userID ID, req *CreateCourseReq) (*CourseCreateResp, error)
	// GenerateOutline 生成大纲：读取课程 + 参考文档文本 → LLM 产出标题与环节列表，
	// 持久化 outline（status=draft）并回填课程标题，返回结果供 handler 逐条 SSE 推送。
	GenerateOutline(ctx context.Context, userID, courseID ID) (*OutlineResult, error)
	// GetOutline 返回某课程已保存的大纲（含 status），无则返回 ErrOutlineNotFound。
	GetOutline(ctx context.Context, userID, courseID ID) (*OutlineView, error)
}
