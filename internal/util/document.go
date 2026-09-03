package util

import (
	"strings"

	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// 文档上传与「纯文本提取」相关约束。
// 早期版本仅支持 .txt / .md / .markdown，后续扩展 PDF/Word 时在此扩展。

const (
	// MaxUploadBytes 单个参考文档大小上限（10MB）。
	MaxUploadBytes = 10 << 20
)

// docExts 允许的参考文档扩展名（小写）。
var docExts = map[string]bool{
	".txt":      true,
	".md":       true,
	".markdown": true,
}

// IsSupportedDoc 判断文件名是否为允许的参考文档格式。
func IsSupportedDoc(filename string) bool {
	return docExts[strings.ToLower(ext(filename))]
}

// ExtractText 对上传文档做纯文本提取（txt/md 直接读取）。
// 非支持格式或超限时返回对应业务错误。
func ExtractText(filename string, data []byte) (string, error) {
	if !IsSupportedDoc(filename) {
		return "", types.ErrUnsupportedFile
	}
	if len(data) > MaxUploadBytes {
		return "", types.ErrFileTooLarge
	}
	return string(data), nil
}

// ext 返回小写扩展名（含点）；无扩展名返回空串。
func ext(filename string) string {
	i := strings.LastIndexByte(filename, '.')
	if i < 0 {
		return ""
	}
	return filename[i:]
}
