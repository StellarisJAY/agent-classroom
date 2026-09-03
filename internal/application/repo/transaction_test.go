package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type txTestModel struct {
	ID   uint `gorm:"primarykey"`
	Name string
}

func (txTestModel) TableName() string { return "tx_test_models" }

func runMigrate(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&txTestModel{}))
	t.Cleanup(func() { _ = db.Migrator().DropTable(&txTestModel{}) })
}

func TestTransactionCommit(t *testing.T) {
	db := testDB(t)
	runMigrate(t, db)
	store := NewStore(db)
	ctx := context.Background()

	err := store.Transaction(ctx, func(ctx context.Context) error {
		// 事务内写两条
		if e := withCtx(db, ctx).Create(&txTestModel{Name: "a"}).Error; e != nil {
			return e
		}
		return withCtx(db, ctx).Create(&txTestModel{Name: "b"}).Error
	})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&txTestModel{}).Count(&count).Error)
	require.Equal(t, int64(2), count, "两条都应提交")
}

func TestTransactionRollbackOnError(t *testing.T) {
	db := testDB(t)
	runMigrate(t, db)
	store := NewStore(db)
	ctx := context.Background()

	err := store.Transaction(ctx, func(ctx context.Context) error {
		if e := withCtx(db, ctx).Create(&txTestModel{Name: "a"}).Error; e != nil {
			return e
		}
		return errors.New("boom")
	})
	require.Error(t, err)

	var count int64
	require.NoError(t, db.Model(&txTestModel{}).Count(&count).Error)
	require.Equal(t, int64(0), count, "出错应整体回滚")
}

// withCtx 模拟 repo 内部取数：优先 ctx 中的 tx，否则默认 db
func withCtx(db *gorm.DB, ctx context.Context) *gorm.DB {
	if tx, ok := txFrom(ctx); ok {
		return tx
	}
	return db
}
