package extractor

import (
	"context"
	"log/slog"
	"strings"

	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// Chain 提取链编排：本地直提 plain 文本；pdf/docx 按配置优先 minerU，失败或为空回退本地。
// provider 取值：local | mineru | chain（默认 chain）。
type Chain struct {
	local  *Local
	mineru *Mineru
}

var _ Extractor = (*Chain)(nil)

// NewChain 创建编排链。mineru 为 nil 表示未配置外部服务（chain 自动退化为 local）。
func NewChain(local *Local, mineru *Mineru) *Chain {
	return &Chain{local: local, mineru: mineru}
}

// Extract 按文件类型与配置选择提取链路。
func (c *Chain) Extract(ctx context.Context, filename string, data []byte) (Result, error) {
	if util.IsPlainDoc(filename) {
		return c.local.Extract(ctx, filename, data)
	}

	if c.mineru != nil {
		res, err := c.mineru.Extract(ctx, filename, data)
		if err == nil && strings.TrimSpace(res.Text) != "" {
			return res, nil
		}
		if err != nil {
			// 用户取消/整体超时不降级，保持可感知的失败
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Result{}, ctxErr
			}
			slog.Warn("mineru extract failed, fallback to local", "filename", filename, "error", err)
		} else {
			// 外部服务成功但提取为空（如扫描件可能仍含 OCR 结果之外的纯图），仍保留原文并交给本地再试
			slog.Warn("mineru extract empty, fallback to local", "filename", filename)
		}
	}
	return c.local.Extract(ctx, filename, data)
}
