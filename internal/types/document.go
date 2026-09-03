package types

import (
	"context"
	"time"
)

// ---- 实体 ----

// Document 参考文档实体，对应 document 表。
// Filename 为原始文件名；URL 为对象存储访问路径（预留抽象，本期本地磁盘）。
// 参考文档提取文本不入库，仅在生成流程读取文件后临时使用。
type Document struct {
	ID       ID        `gorm:"type:uuid;primaryKey" json:"id"`
	CourseID ID        `gorm:"type:uuid;not null;index" json:"-"`
	Filename string    `gorm:"not null" json:"filename"`
	URL      string    `gorm:"not null" json:"url"`
	CreateBy *ID       `gorm:"type:uuid" json:"-"`
	CreateAt time.Time `gorm:"not null;default:now()" json:"-"`
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
}
