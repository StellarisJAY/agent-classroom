package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// outlineRepo 实现 types.OutlineRepo。
type outlineRepo struct {
	base
}

var _ types.OutlineRepo = (*outlineRepo)(nil)

// NewOutlineRepo 创建大纲数据访问实现。
func NewOutlineRepo(db *gorm.DB) types.OutlineRepo {
	return &outlineRepo{base: newBase(db)}
}

func (r *outlineRepo) Create(ctx context.Context, o *types.Outline) error {
	if o.ID == types.NilID {
		o.ID = types.NewID()
	}
	now := time.Now()
	if o.CreateAt.IsZero() {
		o.CreateAt = now
	}
	if o.UpdateAt.IsZero() {
		o.UpdateAt = now
	}
	return r.db(ctx).Create(o).Error
}

func (r *outlineRepo) UpdateContentStatus(ctx context.Context, courseID types.ID, content datatypes.JSON, status string, by *types.ID) error {
	now := time.Now()
	res := r.db(ctx).
		Model(&types.Outline{}).
		Where("course_id = ?", courseID).
		Updates(map[string]any{
			"content":   content,
			"status":    status,
			"update_by": by,
			"update_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return types.ErrNotFound
	}
	return nil
}

func (r *outlineRepo) UpdateContentVersion(ctx context.Context, courseID types.ID, content datatypes.JSON, version int, by *types.ID) error {
	now := time.Now()
	res := r.db(ctx).
		Model(&types.Outline{}).
		Where("course_id = ?", courseID).
		Updates(map[string]any{
			"content":   content,
			"status":    types.OutlineStatusDraft,
			"version":   version,
			"update_by": by,
			"update_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return types.ErrNotFound
	}
	return nil
}

func (r *outlineRepo) GetByCourse(ctx context.Context, courseID types.ID) (*types.Outline, error) {
	var o types.Outline
	err := r.db(ctx).Where("course_id = ?", courseID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *outlineRepo) DeleteByCourse(ctx context.Context, courseID types.ID) error {
	return r.db(ctx).Where("course_id = ?", courseID).Delete(&types.Outline{}).Error
}
