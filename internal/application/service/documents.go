package service

import (
	"context"
	"io"
	"strings"

	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// loadDocumentsText 读取某课程全部参考文档并提取纯文本（按预算截断、文件缺失不阻断），
// 供大纲与各环节内容生成共用。参考文档提取文本不入库，仅生成流程临时使用。
func loadDocumentsText(ctx context.Context, docRepo types.DocumentRepo, storage types.Storage, courseID types.ID) (string, error) {
	docs, err := docRepo.ListByCourse(ctx, courseID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	budget := maxDocRunes
	for _, d := range docs {
		if budget <= 0 {
			break
		}
		rc, err := storage.Get(ctx, d.URL)
		if err != nil {
			continue // 文件缺失不阻断整门课生成
		}
		data, rerr := io.ReadAll(rc)
		rc.Close()
		if rerr != nil {
			continue
		}
		text, xerr := util.ExtractText(d.Filename, data)
		if xerr != nil {
			continue
		}
		if budget > 0 && len(text) > budget {
			text = text[:budget]
		}
		budget -= len(text)
		b.WriteString("\n===== 文档: ")
		b.WriteString(d.Filename)
		b.WriteString(" =====\n")
		b.WriteString(text)
	}
	return b.String(), nil
}
