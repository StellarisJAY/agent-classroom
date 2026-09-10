package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// modelConfigRepo 实现 types.ModelConfigRepo。
type modelConfigRepo struct {
	base
}

var _ types.ModelConfigRepo = (*modelConfigRepo)(nil)

// NewModelConfigRepo 创建用户模型配置数据访问实现。
func NewModelConfigRepo(db *gorm.DB) types.ModelConfigRepo {
	return &modelConfigRepo{base: newBase(db)}
}

func (r *modelConfigRepo) Create(ctx context.Context, m *types.UserModelConfig) error {
	if m.ID == types.NilID {
		m.ID = types.NewID()
	}
	now := time.Now()
	if m.CreateAt.IsZero() {
		m.CreateAt = now
	}
	if m.UpdateAt.IsZero() {
		m.UpdateAt = now
	}
	return r.db(ctx).Create(m).Error
}

func (r *modelConfigRepo) Update(ctx context.Context, m *types.UserModelConfig) error {
	res := r.db(ctx).Model(&types.UserModelConfig{}).
		Where("id = ? AND user_id = ?", m.ID, m.UserID).
		Updates(map[string]any{
			"kind":              m.Kind,
			"provider":          m.Provider,
			"model":             m.Model,
			"base_url":          m.BaseURL,
			"api_key_encrypted": m.APIKeyEncrypted,
			"is_default":        m.IsDefault,
			"update_by":         m.UpdateBy,
			"update_at":         time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return types.ErrNotFound
	}
	return nil
}

func (r *modelConfigRepo) GetByID(ctx context.Context, userID, id types.ID) (*types.UserModelConfig, error) {
	var m types.UserModelConfig
	err := r.db(ctx).Where("id = ? AND user_id = ?", id, userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *modelConfigRepo) ListByUser(ctx context.Context, userID types.ID) ([]types.UserModelConfig, error) {
	var list []types.UserModelConfig
	err := r.db(ctx).Where("user_id = ?", userID).Order("update_at DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (r *modelConfigRepo) GetDefaultByKind(ctx context.Context, userID types.ID, kind string) (*types.UserModelConfig, error) {
	var m types.UserModelConfig
	err := r.db(ctx).Where("user_id = ? AND kind = ? AND is_default", userID, kind).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *modelConfigRepo) Delete(ctx context.Context, userID, id types.ID) error {
	res := r.db(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&types.UserModelConfig{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return types.ErrNotFound
	}
	return nil
}

func (r *modelConfigRepo) ClearDefault(ctx context.Context, userID types.ID, kind string) error {
	return r.db(ctx).
		Model(&types.UserModelConfig{}).
		Where("user_id = ? AND kind = ? AND is_default", userID, kind).
		Update("is_default", false).Error
}
