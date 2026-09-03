package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// userRepo 实现 types.UserRepo。
type userRepo struct {
	base
}

var _ types.UserRepo = (*userRepo)(nil)

// NewUserRepo 创建用户数据访问实现。
func NewUserRepo(db *gorm.DB) types.UserRepo {
	return &userRepo{base: newBase(db)}
}

func (r *userRepo) Create(ctx context.Context, u *types.User) error {
	if u.ID == types.NilID {
		u.ID = types.NewID()
	}
	now := time.Now()
	if u.CreateAt.IsZero() {
		u.CreateAt = now
	}
	if u.UpdateAt.IsZero() {
		u.UpdateAt = now
	}
	return r.db(ctx).Create(u).Error
}

func (r *userRepo) GetByAccount(ctx context.Context, account string) (*types.User, error) {
	var u types.User
	err := r.db(ctx).Where("email = ? OR username = ?", account, account).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) GetByID(ctx context.Context, id types.ID) (*types.User, error) {
	var u types.User
	err := r.db(ctx).Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return r.exists(ctx, "username = ?", username)
}

func (r *userRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return r.exists(ctx, "email = ?", email)
}

func (r *userRepo) exists(ctx context.Context, query string, arg any) (bool, error) {
	var count int64
	err := r.db(ctx).Model(&types.User{}).Where(query, arg).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
