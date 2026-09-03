package types

import (
	"context"
	"io"
)

// TransactionManager 数据库事务管理。业务层通过其把多个 repo 操作放入同一事务。
// 属于跨域基础设施，独立存放；各业务领域的 repo/service 接口按其领域组织到对应
// <domain>.go 中（如 user.go 内的 UserRepo / UserService）。
type TransactionManager interface {
	// Transaction 在单个数据库事务内执行 fn：
	// fn 返回 nil → 提交；返回 error → 回滚。
	// 内部 ctx 已携带事务，repo 方法经统一 db(ctx) 取到事务内的连接。
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// Storage 对象存储抽象（预留：本期为本地磁盘实现，未来可换 S3/OSS）。
// key 为存储内的相对路径；url 为客户端可直接访问的路径（如 /uploads/<key>），
// Get 接收 url，由具体后端解析到对应对象。
type Storage interface {
	// Put 写入对象，返回可访问 url。同名覆盖。
	Put(ctx context.Context, key string, r io.Reader) (url string, err error)
	// Get 读取 url 对应对象内容，调用方负责关闭返回的 ReadCloser。
	Get(ctx context.Context, url string) (io.ReadCloser, error)
}
