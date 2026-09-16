package types

import (
	"context"
	"time"
)

// ---- 实体 ----

// User 用户实体，对应 users 表。
// 字段遵循数据库设计：uuidv7 主键、username/email 唯一、password 用 bcrypt。
type User struct {
	ID           ID        `gorm:"type:uuid;primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	CreateBy     *ID       `gorm:"type:uuid" json:"-"`
	CreateAt     time.Time `gorm:"not null;default:now()" json:"-"`
	UpdateBy     *ID       `gorm:"type:uuid" json:"-"`
	UpdateAt     time.Time `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (User) TableName() string { return "users" }

// ---- DTO ----

// UserInfo 暴露给客户端的用户信息（不含 password_hash / 审计字段）
type UserInfo struct {
	ID       ID     `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// RegisterReq 注册请求
type RegisterReq struct {
	Username string `json:"username" binding:"required,min=2,max=32"`
	Email    string `json:"email" binding:"required,email,max=128"`
	Password string `json:"password" binding:"required,min=6,max=72"`
}

// LoginReq 登录请求，account 可为用户名或邮箱
type LoginReq struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResp 登录成功响应：access token + 用户信息
type LoginResp struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

// ---- 业务错误 ----

var (
	// ErrUsernameTaken 用户名已被占用
	ErrUsernameTaken = NewError(CodeConflict, "用户名已被使用")
	// ErrEmailTaken 邮箱已被占用
	ErrEmailTaken = NewError(CodeConflict, "邮箱已被注册")
	// ErrBadCredentials 账号或密码错误（统一提示，避免账号枚举）
	ErrBadCredentials = NewError(CodeUnauthorized, "账号或密码错误")
)

// ---- 接口 ----

// UserRepo 用户数据访问接口
type UserRepo interface {
	// Create 插入新用户。
	Create(ctx context.Context, u *User) error
	// GetByAccount 按用户名或邮箱查询（用于登录），未找到返回 ErrNotFound。
	GetByAccount(ctx context.Context, account string) (*User, error)
	// GetByID 按主键查询，未找到返回 ErrNotFound。
	GetByID(ctx context.Context, id ID) (*User, error)
	// ExistsByUsername 用户名是否已被占用。
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	// ExistsByEmail 邮箱是否已被占用。
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

// UserService 用户业务接口
type UserService interface {
	// Register 注册新用户；冲突时返回业务错误。
	Register(ctx context.Context, req RegisterReq) (*UserInfo, error)
	// Login 校验凭证并签发 access token。
	Login(ctx context.Context, req LoginReq) (*LoginResp, error)
	// GetMe 返回当前用户信息。
	GetMe(ctx context.Context, id ID) (*UserInfo, error)
}
