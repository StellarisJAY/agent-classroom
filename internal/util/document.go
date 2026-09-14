package util

import "strings"

// 参考「文档上传」格式约束。提取实现见 internal/model/extractor。

const (
	// MaxUploadBytes 单个参考文档大小上限（10MB）。
	MaxUploadBytes = 10 << 20
)

// docExts 允许的参考文档扩展名（小写）。
var docExts = map[string]bool{
	".txt":      true,
	".md":       true,
	".markdown": true,
	".pdf":      true,
	".docx":     true,
}

// IsSupportedDoc 判断文件名是否为允许的参考文档格式。
func IsSupportedDoc(filename string) bool {
	return docExts[strings.ToLower(ext(filename))]
}

// IsPlainDoc 判断文件名是否为可直接读取的纯文本格式（提取器本地首选）。
func IsPlainDoc(filename string) bool {
	switch strings.ToLower(ext(filename)) {
	case ".txt", ".md", ".markdown":
		return true
	}
	return false
}

// IsBinaryDoc 判断文件名是否需要 pdf/word 解析或外部提取服务。
func IsBinaryDoc(filename string) bool {
	switch strings.ToLower(ext(filename)) {
	case ".pdf", ".docx":
		return true
	}
	return false
}

// ext 返回小写扩展名（含点）；无扩展名返回空串。
func ext(filename string) string {
	i := strings.LastIndexByte(filename, '.')
	if i < 0 {
		return ""
	}
	return filename[i:]
}


