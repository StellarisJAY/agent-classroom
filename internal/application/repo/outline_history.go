package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// outlineHistoryRepo 实现 types.OutlineHistoryRepo。
type outlineHistoryRepo struct {
	base
}

var _ types.OutlineHistoryRepo = (*outlineHistoryRepo)(nil)

// NewOutlineHistoryRepo 创建大纲历史版本数据访问实现。
func NewOutlineHistoryRepo(db *gorm.DB) types.OutlineHistoryRepo {
	return &outlineHistoryRepo{base: newBase(db)}
}

func (r *outlineHistoryRepo) Create(ctx context.Context, h *types.OutlineHistory) error {
	if h.ID == types.NilID {
		h.ID = types.NewID()
	}
	if h.CreateAt.IsZero() {
		h.CreateAt = time.Now()
	}
	return r.db(ctx).Create(h).Error
}

func (r *outlineHistoryRepo) ListByOutline(ctx context.Context, outlineID types.ID) ([]types.OutlineHistory, error) {
	var rows []types.OutlineHistory
	err := r.db(ctx).
		Where("outline_id = ?", outlineID).
		Order("version DESC").
		Find(&rows).Error
	return rows, err
}

func (r *outlineHistoryRepo) GetByVersion(ctx context.Context, outlineID types.ID, version int) (*types.OutlineHistory, error) {
	var h types.OutlineHistory
	err := r.db(ctx).
		Where("outline_id = ? AND version = ?", outlineID, version).
		First(&h).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// Prune 仅保留版本号最大的 keep 条快照。
func (r *outlineHistoryRepo) Prune(ctx context.Context, outlineID types.ID, keep int) error {
	if keep <= 0 {
		return nil
	}
	sub := r.db(ctx).
		Model(&types.OutlineHistory{}).
		Select("version").
		Where("outline_id = ?", outlineID).
		Order("version DESC").
		Limit(keep)
	return r.db(ctx).
		Where("outline_id = ? AND version NOT IN (?)", outlineID, sub).
		Delete(&types.OutlineHistory{}).Error
}
