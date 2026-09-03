package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

type mockUserRepo struct {
	create      func(*types.User) error
	byAccount   func(string) (*types.User, error)
	byID        func(types.ID) (*types.User, error)
	existsUser  func(string) (bool, error)
	existsEmail func(string) (bool, error)
}

var _ types.UserRepo = (*mockUserRepo)(nil)

func (m *mockUserRepo) Create(_ context.Context, u *types.User) error { return m.create(u) }
func (m *mockUserRepo) GetByAccount(_ context.Context, a string) (*types.User, error) {
	return m.byAccount(a)
}
func (m *mockUserRepo) GetByID(_ context.Context, id types.ID) (*types.User, error) {
	return m.byID(id)
}
func (m *mockUserRepo) ExistsByUsername(_ context.Context, u string) (bool, error) {
	return m.existsUser(u)
}
func (m *mockUserRepo) ExistsByEmail(_ context.Context, e string) (bool, error) {
	return m.existsEmail(e)
}

func newTestSvc(repo types.UserRepo) types.UserService {
	return NewUserService(repo, &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret-32-byte-key-1234567890", ExpireHours: 1},
	})
}

func TestRegisterSuccess(t *testing.T) {
	svc := newTestSvc(&mockUserRepo{
		existsUser:  func(string) (bool, error) { return false, nil },
		existsEmail: func(string) (bool, error) { return false, nil },
		create:      func(u *types.User) error { u.ID = types.NewID(); return nil },
	})
	info, err := svc.Register(context.Background(), &types.RegisterReq{
		Username: "alice", Email: "alice@example.com", Password: "secret123",
	})
	require.NoError(t, err)
	require.Equal(t, "alice", info.Username)
	require.Equal(t, "alice@example.com", info.Email)
}

func TestRegisterUsernameTaken(t *testing.T) {
	svc := newTestSvc(&mockUserRepo{
		existsUser:  func(string) (bool, error) { return true, nil },
		existsEmail: func(string) (bool, error) { return false, nil },
	})
	_, err := svc.Register(context.Background(), &types.RegisterReq{
		Username: "alice", Email: "other@example.com", Password: "secret123",
	})
	require.ErrorIs(t, err, types.ErrUsernameTaken)
}

func TestRegisterEmailTaken(t *testing.T) {
	svc := newTestSvc(&mockUserRepo{
		existsUser:  func(string) (bool, error) { return false, nil },
		existsEmail: func(string) (bool, error) { return true, nil },
	})
	_, err := svc.Register(context.Background(), &types.RegisterReq{
		Username: "alice", Email: "alice@example.com", Password: "secret123",
	})
	require.ErrorIs(t, err, types.ErrEmailTaken)
}

func TestLoginSuccessByUsername(t *testing.T) {
	id := types.NewID()
	hash := bcryptHash(t, "secret123")
	svc := newTestSvc(&mockUserRepo{
		byAccount: func(string) (*types.User, error) {
			return &types.User{ID: id, Username: "alice", Email: "alice@example.com", PasswordHash: hash}, nil
		},
	})
	resp, err := svc.Login(context.Background(), &types.LoginReq{Account: "alice", Password: "secret123"})
	require.NoError(t, err)
	require.Equal(t, id, resp.User.ID)
	// token 可被解析
	claims, err := util.ParseToken("test-secret-32-byte-key-1234567890", resp.Token)
	require.NoError(t, err)
	require.Equal(t, id, claims.UserID)
}

func TestLoginByEmail(t *testing.T) {
	id := types.NewID()
	hash := bcryptHash(t, "secret123")
	svc := newTestSvc(&mockUserRepo{
		byAccount: func(account string) (*types.User, error) {
			require.Equal(t, "alice@example.com", account)
			return &types.User{ID: id, PasswordHash: hash}, nil
		},
	})
	_, err := svc.Login(context.Background(), &types.LoginReq{Account: "alice@example.com", Password: "secret123"})
	require.NoError(t, err)
}

func TestLoginWrongCredentials(t *testing.T) {
	id := types.NewID()
	hash := bcryptHash(t, "secret123")
	tests := []struct {
		name    string
		byErr   error
		account string
		pass    string
	}{
		{name: "wrong password", account: "alice", pass: "wrong", byErr: nil},
		{name: "account not found", account: "ghost", pass: "secret123", byErr: types.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestSvc(&mockUserRepo{
				byAccount: func(string) (*types.User, error) {
					if tt.byErr != nil {
						return nil, tt.byErr
					}
					return &types.User{ID: id, PasswordHash: hash}, nil
				},
			})
			_, err := svc.Login(context.Background(), &types.LoginReq{Account: tt.account, Password: tt.pass})
			require.ErrorIs(t, err, types.ErrBadCredentials)
		})
	}
}

func bcryptHash(t *testing.T, pwd string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}
