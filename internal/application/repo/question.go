package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// questionRepo 实现 types.QuestionRepo。
type questionRepo struct {
	base
}

var _ types.QuestionRepo = (*questionRepo)(nil)

// NewQuestionRepo 创建测试题数据访问实现。
func NewQuestionRepo(db *gorm.DB) types.QuestionRepo {
	return &questionRepo{base: newBase(db)}
}

// ReplaceBySection 事务内整表替换某环节题目：先删除旧题再批量插入，保证幂等。
func (r *questionRepo) ReplaceBySection(ctx context.Context, sectionID types.ID, questions []types.Question) error {
	return r.database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("section_id = ?", sectionID).Delete(&types.Question{}).Error; err != nil {
			return err
		}
		if len(questions) == 0 {
			return nil
		}
		now := time.Now()
		for i := range questions {
			q := &questions[i]
			q.SectionID = sectionID
			if q.ID == types.NilID {
				q.ID = types.NewID()
			}
			if q.CreateAt.IsZero() {
				q.CreateAt = now
			}
			if q.UpdateAt.IsZero() {
				q.UpdateAt = now
			}
		}
		return tx.Create(&questions).Error
	})
}

func (r *questionRepo) ListBySection(ctx context.Context, sectionID types.ID) ([]types.Question, error) {
	var out []types.Question
	err := r.db(ctx).
		Where("section_id = ?", sectionID).
		Order("position ASC").
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}
