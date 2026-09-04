package types

import (
	"context"
	"time"

	"gorm.io/datatypes"
)

// ---- 测试题类型 ----

// QuestionType 测试题类型。
const (
	// QuestionTypeSingle 单选
	QuestionTypeSingle = "single"
	// QuestionTypeMultiple 多选
	QuestionTypeMultiple = "multiple"
)

// ---- 实体 ----

// Question 测试题实体，对应 question 表（quiz 环节的题目）。
// Options / Answers / Explanations 各为 jsonb 数组：options 为选项文本，
// answers 存正确选项下标（0-based），explanations 为逐选项解析（长度同 options）。
// 见 docs/数据库设计.md 3.6 与需求方案.md 3.2。
type Question struct {
	ID        ID             `gorm:"type:uuid;primaryKey" json:"id"`
	SectionID ID             `gorm:"type:uuid;not null;index" json:"-"`
	Position  int            `gorm:"not null" json:"position"`
	Type      string         `gorm:"type:question_type;not null" json:"type"`
	Stem      string         `gorm:"type:text;not null" json:"stem"`
	Options   datatypes.JSON `gorm:"type:jsonb;not null" json:"-"`
	Answers   datatypes.JSON `gorm:"type:jsonb;not null" json:"-"`
	// Explanations 每选项解析（与 options 一一对应）。
	Explanations datatypes.JSON `gorm:"type:jsonb;not null" json:"-"`
	CreateBy     *ID            `gorm:"type:uuid" json:"-"`
	CreateAt     time.Time      `gorm:"not null;default:now()" json:"-"`
	UpdateBy     *ID            `gorm:"type:uuid" json:"-"`
	UpdateAt     time.Time      `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (Question) TableName() string { return "question" }

// LearnQuestion 学习视图返回的题目 DTO（平铺字段，对齐前端 Question）。
type LearnQuestion struct {
	ID           ID       `json:"id"`
	Position     int      `json:"position"`
	Type         string   `json:"type"`
	Stem         string   `json:"stem"`
	Options      []string `json:"options"`
	Answers      []int    `json:"answers"`
	Explanations []string `json:"explanations"`
}

// ---- 接口 ----

// QuestionRepo 测试题数据访问接口。
type QuestionRepo interface {
	// ReplaceBySection 事务内整表替换某环节的题目（先删后插），保证幂等续跑不产生重复。
	ReplaceBySection(ctx context.Context, sectionID ID, questions []Question) error
	// ListBySection 返回某环节题目，按 position 升序。
	ListBySection(ctx context.Context, sectionID ID) ([]Question, error)
}
