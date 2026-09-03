package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

type ctxTxKey struct{}

// WithTx 将事务 db 写入 context。
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, ctxTxKey{}, tx)
}

// txFrom 从 context 取出事务 db；无事务时返回 false。
func txFrom(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(ctxTxKey{}).(*gorm.DB)
	return tx, ok && tx != nil
}

// base 供各 repo 嵌入，统一取数逻辑。
type base struct {
	database *gorm.DB
}

func newBase(db *gorm.DB) base {
	return base{database: db}
}

// db 返回事务内的连接；若 ctx 无事务则退回默认 db。
func (b base) db(ctx context.Context) *gorm.DB {
	if tx, ok := txFrom(ctx); ok {
		return tx
	}
	return b.database
}

// Store 实现 types.TransactionManager。
type Store struct {
	database *gorm.DB
}

var _ types.TransactionManager = (*Store)(nil)

// NewStore 创建事务管理器。仅暴露 Transaction 能力。
func NewStore(db *gorm.DB) types.TransactionManager {
	return &Store{database: db}
}

// Transaction 在单个数据库事务内执行 fn。
// GORM 语义：fn 返回 error 自动回滚；返回 nil 提交；panic 时 recover 回滚后重抛。
// 嵌套调用自动走 SavePoint。
func (s *Store) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.database.Transaction(func(tx *gorm.DB) error {
		return fn(WithTx(ctx, tx))
	})
}
