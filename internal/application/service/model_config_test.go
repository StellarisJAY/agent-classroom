package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

type mockModelConfigRepo struct {
	create       func(*types.UserModelConfig) error
	update       func(*types.UserModelConfig) error
	byID         func(types.ID, types.ID) (*types.UserModelConfig, error)
	listByUser   func(types.ID) ([]types.UserModelConfig, error)
	getDefault   func(types.ID) (*types.UserModelConfig, error)
	delete       func(types.ID, types.ID) error
	clearDefault func(types.ID) error
}

var _ types.ModelConfigRepo = (*mockModelConfigRepo)(nil)

func (m *mockModelConfigRepo) Create(_ context.Context, c *types.UserModelConfig) error {
	if c.ID == types.NilID {
		c.ID = types.NewID()
	}
	return m.create(c)
}
func (m *mockModelConfigRepo) Update(_ context.Context, c *types.UserModelConfig) error {
	return m.update(c)
}
func (m *mockModelConfigRepo) GetByID(_ context.Context, u, id types.ID) (*types.UserModelConfig, error) {
	return m.byID(u, id)
}
func (m *mockModelConfigRepo) ListByUser(_ context.Context, u types.ID) ([]types.UserModelConfig, error) {
	return m.listByUser(u)
}
func (m *mockModelConfigRepo) GetDefaultByKind(_ context.Context, u types.ID, _ string) (*types.UserModelConfig, error) {
	return m.getDefault(u)
}
func (m *mockModelConfigRepo) Delete(_ context.Context, u, id types.ID) error {
	return m.delete(u, id)
}
func (m *mockModelConfigRepo) ClearDefault(_ context.Context, u types.ID, _ string) error {
	return m.clearDefault(u)
}

// fakeTM 立即执行 fn，模拟单事务直通。
type fakeTM struct{}

func (fakeTM) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newModelConfigSvc(repo types.ModelConfigRepo) types.ModelConfigService {
	cipher, err := util.NewGCMCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		panic(err)
	}
	cfg := &config.Config{}
	cfg.Model.Default = config.ModelDefaultConfig{
		Provider: "openai",
		Model:    "fallback-model",
		BaseURL:  "https://api.openai.com/v1",
		APIKey:   "sk-fallback",
	}
	return NewModelConfigService(repo, fakeTM{}, cipher, cfg)
}

func TestModelConfigCreateEncryptsKey(t *testing.T) {
	var stored *types.UserModelConfig
	svc := newModelConfigSvc(&mockModelConfigRepo{
		create: func(c *types.UserModelConfig) error {
			stored = c
			return nil
		},
	})
	uid := types.NewID()
	info, err := svc.Create(context.Background(), uid, &types.CreateModelConfigReq{
		Provider: "deepseek", Model: "deepseek-chat",
		BaseURL: "https://api.deepseek.com/v1", APIKey: "sk-abcdefgh12345678",
	})
	require.NoError(t, err)
	// 落库为密文
	require.NotEqual(t, "sk-abcdefgh12345678", stored.APIKeyEncrypted)
	require.Equal(t, uid, stored.UserID)
	require.NotEqual(t, types.NilID, stored.ID)
	// 返回掩码
	require.Equal(t, "sk-****5678", info.APIKeyMasked)
	require.NotContains(t, stored.APIKeyEncrypted, "abcdefgh")
}

func TestModelConfigCreateAsDefaultClearsOthers(t *testing.T) {
	cleared := false
	svc := newModelConfigSvc(&mockModelConfigRepo{
		create: func(c *types.UserModelConfig) error { return nil },
		clearDefault: func(types.ID) error {
			cleared = true
			return nil
		},
	})
	_, err := svc.Create(context.Background(), types.NewID(), &types.CreateModelConfigReq{
		Provider: "openai", Model: "gpt-4o-mini", BaseURL: "https://api.openai.com/v1",
		APIKey: "sk-x", IsDefault: true,
	})
	require.NoError(t, err)
	require.True(t, cleared, "设为默认时应先清理旧默认")
}

func TestModelConfigInvalidBaseURL(t *testing.T) {
	svc := newModelConfigSvc(&mockModelConfigRepo{})
	_, err := svc.Create(context.Background(), types.NewID(), &types.CreateModelConfigReq{
		Provider: "openai", Model: "gpt-4o-mini", BaseURL: "api.openai.com", APIKey: "sk-x",
	})
	require.ErrorIs(t, err, types.ErrInvalidBaseURL)
}

func TestModelConfigSetDefaultUnknown(t *testing.T) {
	svc := newModelConfigSvc(&mockModelConfigRepo{
		byID: func(types.ID, types.ID) (*types.UserModelConfig, error) {
			return nil, types.ErrNotFound
		},
	})
	err := svc.SetDefault(context.Background(), types.NewID(), types.NewID())
	require.ErrorIs(t, err, types.ErrModelConfigNotFound)
}

func TestModelConfigSetDefaultClearsAndPromotes(t *testing.T) {
	uid, id := types.NewID(), types.NewID()
	cleared := false
	svc := newModelConfigSvc(&mockModelConfigRepo{
		byID: func(u, i types.ID) (*types.UserModelConfig, error) {
			return &types.UserModelConfig{ID: id, UserID: uid}, nil
		},
		clearDefault: func(types.ID) error {
			cleared = true
			return nil
		},
		update: func(c *types.UserModelConfig) error {
			require.True(t, c.IsDefault)
			return nil
		},
	})
	err := svc.SetDefault(context.Background(), uid, id)
	require.NoError(t, err)
	require.True(t, cleared)
}

func TestModelConfigUpdateKeepsKeyWhenEmpty(t *testing.T) {
	uid, id := types.NewID(), types.NewID()
	enc := "existing-cipher"
	svc := newModelConfigSvc(&mockModelConfigRepo{
		byID: func(u, i types.ID) (*types.UserModelConfig, error) {
			return &types.UserModelConfig{ID: id, UserID: uid, Provider: "openai",
				Model: "old", BaseURL: "https://old", APIKeyEncrypted: enc}, nil
		},
		update: func(c *types.UserModelConfig) error {
			require.Equal(t, enc, c.APIKeyEncrypted, "api_key 留空不应覆盖密文")
			require.Equal(t, "new-model", c.Model)
			return nil
		},
	})
	_, err := svc.Update(context.Background(), uid, id, &types.UpdateModelConfigReq{
		Model: "new-model", APIKey: "",
	})
	require.NoError(t, err)
}

func TestModelConfigUpdateDefaultKindChangeClearsOldKind(t *testing.T) {
	uid, id := types.NewID(), types.NewID()
	cleared := false
	svc := newModelConfigSvc(&mockModelConfigRepo{
		byID: func(u, i types.ID) (*types.UserModelConfig, error) {
			return &types.UserModelConfig{ID: id, UserID: uid, Kind: "llm",
				IsDefault: true, Provider: "openai", Model: "m", BaseURL: "https://x"}, nil
		},
		clearDefault: func(types.ID) error {
			cleared = true
			return nil
		},
		update: func(c *types.UserModelConfig) error { return nil },
	})
	_, err := svc.Update(context.Background(), uid, id, &types.UpdateModelConfigReq{Kind: "image"})
	require.NoError(t, err)
	require.True(t, cleared, "已是默认的配置变更 kind 时应清除旧 kind 默认")
}

func TestModelConfigDeleteUnknown(t *testing.T) {
	svc := newModelConfigSvc(&mockModelConfigRepo{
		delete: func(types.ID, types.ID) error {
			return types.ErrNotFound
		},
	})
	err := svc.Delete(context.Background(), types.NewID(), types.NewID())
	require.ErrorIs(t, err, types.ErrModelConfigNotFound)
}

func TestModelConfigListDecryptsMask(t *testing.T) {
	cipher, _ := util.NewGCMCipher([]byte("0123456789abcdef0123456789abcdef"))
	enc, err := cipher.Encrypt("sk-abcdefghijkl5678")
	require.NoError(t, err)
	uid, id := types.NewID(), types.NewID()
	svc := newModelConfigSvc(&mockModelConfigRepo{
		listByUser: func(types.ID) ([]types.UserModelConfig, error) {
			return []types.UserModelConfig{{ID: id, UserID: uid, Provider: "qwen",
				Model: "qwen-max", BaseURL: "https://dashscope", APIKeyEncrypted: enc}}, nil
		},
	})
	infos, err := svc.List(context.Background(), uid)
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "sk-****5678", infos[0].APIKeyMasked)
}

func TestModelConfigResolveDefaultFromUserConfig(t *testing.T) {
	cipher, _ := util.NewGCMCipher([]byte("0123456789abcdef0123456789abcdef"))
	enc, err := cipher.Encrypt("sk-user-key")
	require.NoError(t, err)
	uid := types.NewID()
	svc := newModelConfigSvc(&mockModelConfigRepo{
		getDefault: func(types.ID) (*types.UserModelConfig, error) {
			return &types.UserModelConfig{
				UserID: uid, Provider: "deepseek", Model: "deepseek-chat",
				BaseURL: "https://api.deepseek.com/v1", APIKeyEncrypted: enc,
			}, nil
		},
	})
	pc, err := svc.ResolveDefault(context.Background(), uid)
	require.NoError(t, err)
	require.Equal(t, "deepseek", pc.Provider)
	require.Equal(t, "sk-user-key", pc.APIKey)
}

func TestModelConfigResolveDefaultFallback(t *testing.T) {
	uid := types.NewID()
	svc := newModelConfigSvc(&mockModelConfigRepo{
		getDefault: func(types.ID) (*types.UserModelConfig, error) {
			return nil, types.ErrNotFound
		},
	})
	pc, err := svc.ResolveDefault(context.Background(), uid)
	require.NoError(t, err)
	require.Equal(t, "openai", pc.Provider)
	require.Equal(t, "fallback-model", pc.Model)
	require.Equal(t, "sk-fallback", pc.APIKey)
}

func TestModelConfigResolveByIDNotFound(t *testing.T) {
	svc := newModelConfigSvc(&mockModelConfigRepo{
		byID: func(types.ID, types.ID) (*types.UserModelConfig, error) {
			return nil, types.ErrNotFound
		},
	})
	_, err := svc.ResolveByID(context.Background(), types.NewID(), types.NewID())
	require.ErrorIs(t, err, types.ErrModelConfigNotFound)
}
