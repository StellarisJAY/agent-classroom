package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// documentRepo 实现 types.DocumentRepo。
type documentRepo struct {
	base
}

var _ types.DocumentRepo = (*documentRepo)(nil)

// NewDocumentRepo 创建参考文档数据访问实现。
func NewDocumentRepo(db *gorm.DB) types.DocumentRepo {
	return &documentRepo{base: newBase(db)}
}

func (r *documentRepo) Create(ctx context.Context, d *types.Document) error {
	if d.ID == types.NilID {
		d.ID = types.NewID()
	}
	if d.CreateAt.IsZero() {
		d.CreateAt = time.Now()
	}
	return r.db(ctx).Create(d).Error
}

func (r *documentRepo) ListByCourse(ctx context.Context, courseID types.ID) ([]types.Document, error) {
	var list []types.Document
	err := r.db(ctx).Where("course_id = ?", courseID).Order("create_at ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}
