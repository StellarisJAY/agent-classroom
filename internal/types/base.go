package types

import "github.com/google/uuid"

// ID 主键/关联键的统一类型别名。
// 业务实体、DTO 与接口签名统一使用 ID，避免各层显式依赖 uuid.UUID，
// 便于未来整体切换底层 ID 实现。别名保留 uuid.UUID 的方法，json/gorm 序列化行为不变。
type ID = uuid.UUID

// NilID 零值 ID，等价于 uuid.Nil
var NilID = uuid.Nil

// ParseID 将字符串解析为 ID，供 handler 解析路径参数等使用。
func ParseID(s string) (ID, error) { return uuid.Parse(s) }

// MustParseID 解析字符串为 ID，失败时 panic，用于常量或测试中的硬编码。
func MustParseID(s string) ID { return uuid.Must(uuid.Parse(s)) }

// NewID 生成新的 ID（UUIDv7，按时间有序），供应用层新建记录使用。
func NewID() ID {
	id, err := uuid.NewV7()
	if err != nil {
		panic("types.NewID: generate uuid v7: " + err.Error())
	}
	return id
}
