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

func (s *ModelConfigService) Create(ctx context.Context, userID types.ID, req types.CreateModelConfigReq) (*types.ModelConfigInfo, error) {
	if err := validateBaseURL(req.BaseURL); err != nil {
		return nil, err
	}
	encrypted, err := s.cipher.Encrypt(req.APIKey)
	if err != nil {
		return nil, err
	}
	kind := normalizeKind(req.Kind)
	m := &types.UserModelConfig{
		UserID:          userID,
		Kind:            kind,
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
			if err := s.repo.ClearDefault(ctx, userID, kind); err != nil {
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
		ID: m.ID, Kind: m.Kind, Provider: m.Provider, Model: m.Model, BaseURL: m.BaseURL,
		APIKeyMasked: maskKey(req.APIKey), IsDefault: m.IsDefault,
	}, nil
}

func (s *ModelConfigService) Update(ctx context.Context, userID, id types.ID, req types.UpdateModelConfigReq) (*types.ModelConfigInfo, error) {
	cur, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrModelConfigNotFound
		}
		return nil, err
	}
	origKind := cur.Kind
	// 覆盖可选字段
	if req.Kind != "" {
		cur.Kind = normalizeKind(req.Kind)
	}
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
			if err := s.repo.ClearDefault(ctx, userID, normalizeKind(cur.Kind)); err != nil {
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
		// 已是默认的配置变更 kind 时，旧 kind 的默认标记需一并清除，避免默认串 kind。
		if cur.IsDefault && req.Kind != "" && normalizeKind(origKind) != normalizeKind(req.Kind) {
			oldKind := origKind
			err = s.tm.Transaction(ctx, func(ctx context.Context) error {
				cur.IsDefault = req.IsDefault
				cur.UpdateAt = time.Now()
				if err := s.repo.Update(ctx, cur); err != nil {
					return err
				}
				return s.repo.ClearDefault(ctx, userID, normalizeKind(oldKind))
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
		if err := s.repo.ClearDefault(ctx, userID, normalizeKind(cur.Kind)); err != nil {
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
		ID: m.ID, Kind: normalizeKind(m.Kind), Provider: m.Provider, Model: m.Model, BaseURL: m.BaseURL,
		APIKeyMasked: maskKey(plain), IsDefault: m.IsDefault,
	}, nil
}

// ResolveDefault 解析平台默认 LLM 模型为可用 ProviderConfig；无默认项时返回 ErrNoModelConfig。
func (s *ModelConfigService) ResolveDefault(ctx context.Context, userID types.ID) (model.ProviderConfig, error) {
	return s.ResolveDefaultByKind(ctx, userID, types.ModelKindLLM)
}

// ResolveDefaultByKind 解析指定用途的兜底模型（配置 model.options 中该 kind 的 default: true 项）。
// 平台不向用户开放模型配置，故始终只读配置文件，不查用户库。
// 无匹配默认项时返回 ErrNoModelConfig（image 通常不设默认，未显式选择则不生成配图）。
func (s *ModelConfigService) ResolveDefaultByKind(_ context.Context, _ types.ID, kind string) (model.ProviderConfig, error) {
	for _, o := range s.cfg.Model.Options {
		if normalizeKind(o.Kind) == normalizeKind(kind) && o.Default {
			return s.toProviderConfig(o), nil
		}
	}
	return model.ProviderConfig{}, types.ErrNoModelConfig
}

// Options 返回配置文件中的平台全局可选模型清单（不含密钥）。
func (s *ModelConfigService) Options(_ context.Context) []types.ModelOptionInfo {
	out := make([]types.ModelOptionInfo, 0, len(s.cfg.Model.Options))
	for _, o := range s.cfg.Model.Options {
		if o.Key == "" {
			continue
		}
		label := o.Label
		if label == "" {
			label = o.Model
		}
		out = append(out, types.ModelOptionInfo{
			Key:       o.Key,
			Kind:      normalizeKind(o.Kind),
			Label:     label,
			IsDefault: o.Default,
			Provider:  o.Provider,
			Model:     o.Model,
		})
	}
	return out
}

// ResolveByKey 按配置文件中的 option key 解析为 ProviderConfig；未找到返回 ErrModelConfigNotFound。
func (s *ModelConfigService) ResolveByKey(_ context.Context, key string) (model.ProviderConfig, error) {
	for _, o := range s.cfg.Model.Options {
		if o.Key == key {
			return s.toProviderConfig(o), nil
		}
	}
	return model.ProviderConfig{}, types.ErrModelConfigNotFound
}

// toProviderConfig 将配置项组装为可用的 ProviderConfig（补全全局超时设置）。
func (s *ModelConfigService) toProviderConfig(o config.ModelOptionConfig) model.ProviderConfig {
	return model.ProviderConfig{
		Provider:      o.Provider,
		Model:         o.Model,
		BaseURL:       o.BaseURL,
		APIKey:        o.APIKey,
		Timeout:       s.cfg.Model.Timeout,
		StreamTimeout: s.cfg.Model.StreamTimeout,
	}
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

// normalizeKind 将空 kind 归一为 llm。
func normalizeKind(kind string) string {
	if kind == "" {
		return types.ModelKindLLM
	}
	return kind
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
