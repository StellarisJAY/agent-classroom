package extractor

import (
	"context"
)

// Result 单文档提取结果。
type Result struct {
	// Text 提取出的文本（可能为空，如扫描版 PDF 的文本层为空）
	Text string
	// Source 使用的提取器来源：local | mineru
	Source string
}

// Extractor 参考文档提取器接口。
// 实现需保证幂等：同一输入可重复调用；耗时提取应在 ctx 取消/超时时返回错误。
type Extractor interface {
	// Extract 提取文档纯文本。filename 用于选择解析方式。
	Extract(ctx context.Context, filename string, data []byte) (Result, error)
}
