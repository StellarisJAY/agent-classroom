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

// ---- 实体 ----

// Course 课程实体，对应 course 表。
type Course struct {
	ID          ID        `gorm:"type:uuid;primaryKey" json:"id"`
	OwnerID     ID        `gorm:"type:uuid;not null" json:"-"`
	Title       string    `gorm:"not null" json:"title"`
	Description string    `gorm:"not null" json:"description"`
	Status      string    `gorm:"type:course_status;not null;default:draft" json:"status"`
	IsPublic    bool      `gorm:"not null;default:false" json:"is_public"`
	CreateBy    *ID       `gorm:"type:uuid" json:"-"`
	CreateAt    time.Time `gorm:"not null;default:now()" json:"-"`
	UpdateAt    time.Time `gorm:"not null;default:now()" json:"-"`
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
	ID          ID     `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	IsPublic    bool   `json:"is_public"`
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

// ---- 业务错误 ----

var (
	// ErrInvalidScope 归属筛选取值不合法
	ErrInvalidScope = NewError(CodeBadRequest, "归属筛选不合法")
	// ErrCourseNotFound 课程不存在或无访问权限
	ErrCourseNotFound = NewError(CodeNotFound, "课程不存在")
)

// ---- 接口 ----

// CourseRepo 课程数据访问接口。
type CourseRepo interface {
	// List 按过滤条件分页查询当前用户可见课程，返回行列表与命中总数。
	// 行内含 LEFT JOIN progress 得到的当前用户进度（无记录时 progress_status 为空串）。
	List(ctx context.Context, userID ID, f CourseListFilter) ([]CourseListRow, int64, error)
}

// CourseService 课程业务接口。
type CourseService interface {
	// List 分页返回当前用户课程列表；req 为 nil 或缺省字段时取默认（all / 第 1 页 / 每页 20）。
	List(ctx context.Context, userID ID, req *CourseListReq) (*CourseListResp, error)
}
