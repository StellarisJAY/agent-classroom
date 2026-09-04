package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// ModelConfigService 用户模型配置业务实现。
type ModelConfigService struct {
	repo   types.ModelConfigRepo
	tm     types.TransactionManager
	cipher *util.GCMCipher
	cfg    *config.Config
}

var _ types.ModelConfigService = (*ModelConfigService)(nil)

// NewModelConfigService 创建用户模型配置业务实现。
// tm 用于默认配置互斥的原子事务；cipher 用于 API Key 加解密；cfg 提供兜底默认模型。
func NewModelConfigService(repo types.ModelConfigRepo, tm types.TransactionManager, cipher *util.GCMCipher, cfg *config.Config) types.ModelConfigService {
	return &ModelConfigService{repo: repo, tm: tm, cipher: cipher, cfg: cfg}
}

func (s *ModelConfigService) List(ctx context.Context, userID types.ID) ([]types.ModelConfigInfo, error) {
	list, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	infos := make([]types.ModelConfigInfo, 0, len(list))
	for i := range list {
		info, err := s.toInfo(ctx, &list[i])
		if err != nil {
			return nil, err
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

func (s *ModelConfigService) Create(ctx context.Context, userID types.ID, req *types.CreateModelConfigReq) (*types.ModelConfigInfo, error) {
	if err := validateBaseURL(req.BaseURL); err != nil {
		return nil, err
	}
	encrypted, err := s.cipher.Encrypt(req.APIKey)
	if err != nil {
		return nil, err
	}
	m := &types.UserModelConfig{
		UserID:          userID,
		Provider:        req.Provider,
		Model:           req.Model,
		BaseURL:         req.BaseURL,
		APIKeyEncrypted: encrypted,
		IsDefault:       req.IsDefault,
		CreateBy:        &userID,
		UpdateBy:        &userID,
	}
	if req.IsDefault {
		err = s.tm.Transaction(ctx, func(ctx context.Context) error {
			if err := s.repo.ClearDefault(ctx, userID); err != nil {
				return err
			}
			return s.repo.Create(ctx, m)
		})
	} else {
		err = s.repo.Create(ctx, m)
	}
	if err != nil {
		return nil, err
	}
	return &types.ModelConfigInfo{
		ID: m.ID, Provider: m.Provider, Model: m.Model, BaseURL: m.BaseURL,
		APIKeyMasked: maskKey(req.APIKey), IsDefault: m.IsDefault,
	}, nil
}

func (s *ModelConfigService) Update(ctx context.Context, userID, id types.ID, req *types.UpdateModelConfigReq) (*types.ModelConfigInfo, error) {
	cur, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrModelConfigNotFound
		}
		return nil, err
	}
	// 覆盖可选字段
	if req.Provider != "" {
		cur.Provider = req.Provider
	}
	if req.Model != "" {
		cur.Model = req.Model
	}
	if req.BaseURL != "" {
		if err := validateBaseURL(req.BaseURL); err != nil {
			return nil, err
		}
		cur.BaseURL = req.BaseURL
	}
	if req.APIKey != "" {
		encrypted, err := s.cipher.Encrypt(req.APIKey)
		if err != nil {
			return nil, err
		}
		cur.APIKeyEncrypted = encrypted
	}
	cur.UpdateBy = &userID

	// 切换为默认时才需跨行清理；否则单行更新。
	if req.IsDefault && !cur.IsDefault {
		err = s.tm.Transaction(ctx, func(ctx context.Context) error {
			if err := s.repo.ClearDefault(ctx, userID); err != nil {
				return err
			}
			cur.IsDefault = true
			cur.UpdateAt = time.Now()
			return s.repo.Update(ctx, cur)
		})
		if err != nil {
			return nil, err
		}
	} else {
		cur.IsDefault = req.IsDefault
		if err := s.repo.Update(ctx, cur); err != nil {
			return nil, err
		}
	}

	return s.toInfo(ctx, cur)
}

func (s *ModelConfigService) Delete(ctx context.Context, userID, id types.ID) error {
	err := s.repo.Delete(ctx, userID, id)
	if errors.Is(err, types.ErrNotFound) {
		return types.ErrModelConfigNotFound
	}
	return err
}

func (s *ModelConfigService) SetDefault(ctx context.Context, userID, id types.ID) error {
	cur, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return types.ErrModelConfigNotFound
		}
		return err
	}
	if cur.IsDefault {
		return nil
	}
	return s.tm.Transaction(ctx, func(ctx context.Context) error {
		if err := s.repo.ClearDefault(ctx, userID); err != nil {
			return err
		}
		cur.IsDefault = true
		cur.UpdateBy = &userID
		cur.UpdateAt = time.Now()
		return s.repo.Update(ctx, cur)
	})
}

// toInfo 解密后掩码 key 生成返回结构；解密失败视为无 key。
func (s *ModelConfigService) toInfo(_ context.Context, m *types.UserModelConfig) (*types.ModelConfigInfo, error) {
	plain := ""
	if m.APIKeyEncrypted != "" {
		if dec, err := s.cipher.Decrypt(m.APIKeyEncrypted); err == nil {
			plain = dec
		}
	}
	return &types.ModelConfigInfo{
		ID: m.ID, Provider: m.Provider, Model: m.Model, BaseURL: m.BaseURL,
		APIKeyMasked: maskKey(plain), IsDefault: m.IsDefault,
	}, nil
}

// ResolveDefault 解析用户默认配置为可用 ProviderConfig；无默认时回退服务端兜底配置。
func (s *ModelConfigService) ResolveDefault(ctx context.Context, userID types.ID) (model.ProviderConfig, error) {
	m, err := s.repo.GetDefault(ctx, userID)
	if err == nil {
		return s.resolveConfig(m)
	}
	if !errors.Is(err, types.ErrNotFound) {
		return model.ProviderConfig{}, err
	}
	d := s.cfg.Model.Default
	return model.ProviderConfig{
		Provider:      d.Provider,
		Model:         d.Model,
		BaseURL:       d.BaseURL,
		APIKey:        d.APIKey,
		Timeout:       s.cfg.Model.Timeout,
		StreamTimeout: s.cfg.Model.StreamTimeout,
	}, nil
}

// ResolveByID 解析指定配置为 ProviderConfig；未找到返回 ErrModelConfigNotFound。
func (s *ModelConfigService) ResolveByID(ctx context.Context, userID, configID types.ID) (model.ProviderConfig, error) {
	m, err := s.repo.GetByID(ctx, userID, configID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return model.ProviderConfig{}, types.ErrModelConfigNotFound
		}
		return model.ProviderConfig{}, err
	}
	return s.resolveConfig(m)
}

// resolveConfig 解密 API Key 并组装 ProviderConfig。
func (s *ModelConfigService) resolveConfig(m *types.UserModelConfig) (model.ProviderConfig, error) {
	key, err := s.cipher.Decrypt(m.APIKeyEncrypted)
	if err != nil {
		return model.ProviderConfig{}, fmt.Errorf("decrypt model api key: %w", err)
	}
	return model.ProviderConfig{
		Provider:      m.Provider,
		Model:         m.Model,
		BaseURL:       m.BaseURL,
		APIKey:        key,
		Timeout:       s.cfg.Model.Timeout,
		StreamTimeout: s.cfg.Model.StreamTimeout,
	}, nil
}

func validateBaseURL(baseURL string) error {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return types.ErrInvalidBaseURL
	}
	return nil
}

func maskKey(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:3] + "****" + secret[len(secret)-4:]
}
