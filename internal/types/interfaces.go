package types

import "context"

// TransactionManager 数据库事务管理。业务层通过其把多个 repo 操作放入同一事务。
// 属于跨域基础设施，独立存放；各业务领域的 repo/service 接口按其领域组织到对应
// <domain>.go 中（如 user.go 内的 UserRepo / UserService）。
type TransactionManager interface {
	// Transaction 在单个数据库事务内执行 fn：
	// fn 返回 nil → 提交；返回 error → 回滚。
	// 内部 ctx 已携带事务，repo 方法经统一 db(ctx) 取到事务内的连接。
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}
