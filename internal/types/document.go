package types

import (
	"context"
	"time"
)

// 文档提取状态（document 表 extracted_status 列）。
const (
	// ExtractStatusPending 尚未提取（新建文档的初始状态）
	ExtractStatusPending = "pending"
	// ExtractStatusSuccess 提取完成，文本已缓存
	ExtractStatusSuccess = "success"
	// ExtractStatusFailed 提取失败（文本为空，不阻断生成）
	ExtractStatusFailed = "failed"
)

// ---- 实体 ----

// Document 参考文档实体，对应 document 表。
// Filename 为原始文件名；URL 为对象存储访问路径（预留抽象，本期本地磁盘）。
// 提取结果缓存于 extracted_text，配合 extracted_status 实现幂等提取与复用。
type Document struct {
	ID              ID        `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID        ID        `gorm:"type:uuid;not null;index" json:"-"`
	Filename        string    `gorm:"not null" json:"filename"`
	URL             string    `gorm:"not null" json:"url"`
	ExtractedStatus string    `gorm:"type:varchar(16);not null;default:'pending'" json:"extractedStatus"`
	ExtractedText   *string   `gorm:"type:text" json:"-"`
	CreateBy        *ID       `gorm:"type:uuid" json:"-"`
	CreateAt        time.Time `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (Document) TableName() string { return "document" }

// ---- 接口 ----

// DocumentRepo 参考文档数据访问接口。
type DocumentRepo interface {
	// Create 插入新文档（ID/时间戳由实现填充）。
	Create(ctx context.Context, d *Document) error
	// ListByCourse 返回某课程的全部文档。
	ListByCourse(ctx context.Context, courseID ID) ([]Document, error)
	// UpdateExtracted 写回单个文档的提取结果（文本 + 状态）。
	UpdateExtracted(ctx context.Context, id ID, status string, text string) error
}
