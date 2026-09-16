package service

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// UserService 用户业务实现。
type UserService struct {
	repo types.UserRepo
	cfg  *config.Config
}

var _ types.UserService = (*UserService)(nil)

// NewUserService 创建用户业务实现。
func NewUserService(repo types.UserRepo, cfg *config.Config) types.UserService {
	return &UserService{repo: repo, cfg: cfg}
}

func (s *UserService) Register(ctx context.Context, req types.RegisterReq) (*types.UserInfo, error) {
	if ok, err := s.repo.ExistsByUsername(ctx, req.Username); err != nil {
		return nil, err
	} else if ok {
		return nil, types.ErrUsernameTaken
	}
	if ok, err := s.repo.ExistsByEmail(ctx, req.Email); err != nil {
		return nil, err
	} else if ok {
		return nil, types.ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &types.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return toUserInfo(user), nil
}

func (s *UserService) Login(ctx context.Context, req types.LoginReq) (*types.LoginResp, error) {
	user, err := s.repo.GetByAccount(ctx, req.Account)
	if err != nil {
		// 账号不存在统一走凭据错误，避免账号枚举
		if err == types.ErrNotFound {
			return nil, types.ErrBadCredentials
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, types.ErrBadCredentials
	}

	expire := time.Duration(s.cfg.JWT.ExpireHours) * time.Hour
	token, err := util.SignToken(s.cfg.JWT.Secret, user.ID, user.Username, expire)
	if err != nil {
		return nil, err
	}
	return &types.LoginResp{Token: token, User: *toUserInfo(user)}, nil
}

func (s *UserService) GetMe(ctx context.Context, id types.ID) (*types.UserInfo, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toUserInfo(user), nil
}

func toUserInfo(u *types.User) *types.UserInfo {
	return &types.UserInfo{ID: u.ID, Username: u.Username, Email: u.Email}
}
