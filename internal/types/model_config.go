package types

import (
	"context"
	"time"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// ---- 实体 ----

// UserModelConfig 用户模型配置实体，对应 user_model_config 表。
// API Key 服务端以 AES-256-GCM 对称加密后落库（api_key_encrypted）。
type UserModelConfig struct {
	ID              ID        `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          ID        `gorm:"type:uuid;not null;index" json:"-"`
	Provider        string    `gorm:"not null" json:"provider"`
	Model           string    `gorm:"not null" json:"model"`
	BaseURL         string    `gorm:"not null" json:"base_url"`
	APIKeyEncrypted string    `gorm:"not null" json:"-"`
	IsDefault       bool      `gorm:"not null;default:false" json:"is_default"`
	CreateBy        *ID       `gorm:"type:uuid" json:"-"`
	CreateAt        time.Time `gorm:"not null;default:now()" json:"-"`
	UpdateBy        *ID       `gorm:"type:uuid" json:"-"`
	UpdateAt        time.Time `gorm:"not null;default:now()" json:"-"`
}

// TableName 指定表名
func (UserModelConfig) TableName() string { return "user_model_config" }

// ---- DTO ----

// ModelConfigInfo 返回给客户端的配置信息（key 掩码展示，不暴露明文）。
type ModelConfigInfo struct {
	ID           ID     `json:"id"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	BaseURL      string `json:"base_url"`
	APIKeyMasked string `json:"api_key_masked"`
	IsDefault    bool   `json:"is_default"`
}

// CreateModelConfigReq 新增模型配置请求
type CreateModelConfigReq struct {
	Provider  string `json:"provider" binding:"required,min=1,max=32"`
	Model     string `json:"model" binding:"required,min=1,max=128"`
	BaseURL   string `json:"base_url" binding:"required,max=256"`
	APIKey    string `json:"api_key" binding:"required,min=1,max=512"`
	IsDefault bool   `json:"is_default"`
}

// UpdateModelConfigReq 编辑模型配置请求。
// api_key 留空表示不修改（避免覆盖已保存的 key）。
type UpdateModelConfigReq struct {
	Provider  string `json:"provider" binding:"omitempty,min=1,max=32"`
	Model     string `json:"model" binding:"omitempty,min=1,max=128"`
	BaseURL   string `json:"base_url" binding:"omitempty,max=256"`
	APIKey    string `json:"api_key" binding:"omitempty,max=512"`
	IsDefault bool   `json:"is_default"`
}

// ---- 业务错误 ----

var (
	// ErrModelConfigNotFound 模型配置不存在或不属于当前用户
	ErrModelConfigNotFound = NewError(CodeNotFound, "模型配置不存在")
	// ErrInvalidBaseURL base_url 必须以 http(s):// 开头
	ErrInvalidBaseURL = NewError(CodeBadRequest, "base_url 需以 http(s):// 开头")
)

// ---- 接口 ----

// ModelConfigRepo 用户模型配置数据访问接口。
// 除 Create 外均按 user_id 限定，保证不能越权访问他人配置。
type ModelConfigRepo interface {
	// Create 插入新配置（ID/时间戳由实现填充）。
	Create(ctx context.Context, m *UserModelConfig) error
	// Update 更新业务字段（provider/model/base_url/api_key_encrypted/is_default 及审计字段）。
	Update(ctx context.Context, m *UserModelConfig) error
	// GetByID 按主键 + user_id 查询，未找到返回 ErrNotFound。
	GetByID(ctx context.Context, userID, id ID) (*UserModelConfig, error)
	// ListByUser 返回某用户全部配置。
	ListByUser(ctx context.Context, userID ID) ([]UserModelConfig, error)
	// GetDefault 返回某用户的默认配置；无默认返回 ErrNotFound。
	GetDefault(ctx context.Context, userID ID) (*UserModelConfig, error)
	// Delete 删除某用户的一条配置；未找到返回 ErrNotFound。
	Delete(ctx context.Context, userID, id ID) error
	// ClearDefault 清除某用户的默认标记（is_default=false）。
	ClearDefault(ctx context.Context, userID ID) error
}

// ModelConfigService 用户模型配置业务接口。
type ModelConfigService interface {
	// List 返回用户全部配置（key 掩码）。
	List(ctx context.Context, userID ID) ([]ModelConfigInfo, error)
	// Create 新增配置；is_default 为真时清除旧的默认标记。
	Create(ctx context.Context, userID ID, req *CreateModelConfigReq) (*ModelConfigInfo, error)
	// Update 编辑配置；api_key 留空不改；切换为默认时保证唯一默认。
	Update(ctx context.Context, userID, id ID, req *UpdateModelConfigReq) (*ModelConfigInfo, error)
	// Delete 删除配置。
	Delete(ctx context.Context, userID, id ID) error
	// SetDefault 将指定配置设为该用户唯一默认。
	SetDefault(ctx context.Context, userID, id ID) error
	// ResolveDefault 解析用户默认配置为可用的 ProviderConfig；无默认时回退服务端兜底配置。
	ResolveDefault(ctx context.Context, userID ID) (model.ProviderConfig, error)
	// ResolveByID 解析指定配置为 ProviderConfig；未找到返回 ErrModelConfigNotFound。
	ResolveByID(ctx context.Context, userID, configID ID) (model.ProviderConfig, error)
}
