package extractor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	pdf "github.com/ledongthuc/pdf"
	ctypes "github.com/gomutex/godocx/wml/ctypes"

	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// Local 本地提取器：txt/md 直读，pdf 走文本层提取，docx 解析段落。
// 扫描版 PDF 无文本层时返回空文本（由上层决定是否走外部服务兜底）。
type Local struct{}

var _ Extractor = (*Local)(nil)

// NewLocal 创建本地提取器。
func NewLocal() *Local { return &Local{} }

// Extract 按扩展名分派本地提取。
func (l *Local) Extract(_ context.Context, filename string, data []byte) (Result, error) {
	switch {
	case util.IsPlainDoc(filename):
		return Result{Text: string(data), Source: "local"}, nil
	case strings.EqualFold(fileExt(filename), ".docx"):
		text, err := extractDocx(data)
		return Result{Text: text, Source: "local"}, err
	}
	return Result{}, fmt.Errorf("local extractor: unsupported file %q", filename)
}
// extractPDF 提取 PDF 文本层（ledongthuc/pdf，仅处理文本层，扫描件返回空文本）。
func extractPDF(data []byte) (string, error) {
	r := bytes.NewReader(data)
	reader, err := pdf.NewReader(r, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf open: %w", err)
	}
	var b strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		content, err := reader.Page(i).GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("pdf extract text: %w", err)
		}
		b.WriteString(content)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// extractDocx 解压 document.xml 并逐个解析 w:p 元素提取段落文本；
// 表格内与文本框内的段落同样以 w:p 出现，统一覆盖。
func extractDocx(data []byte) (string, error) {
	name, files, err := unzipEntry(data, "word/document.xml")
	if err != nil {
		return "", fmt.Errorf("docx unzip: %w", err)
	}
	if name == "" {
		return "", fmt.Errorf("docx unzip: word/document.xml not found")
	}
	dec := xml.NewDecoder(bytes.NewReader(files))
	dec.Strict = false
	var b strings.Builder
	for {
		tok, terr := dec.Token()
		if terr != nil {
			if errors.Is(terr, io.EOF) {
				break
			}
			return "", fmt.Errorf("docx parse: %w", terr)
		}
		if t, ok := tok.(xml.StartElement); ok && t.Name.Local == "p" &&
			strings.HasSuffix(t.Name.Space, "wordprocessingml/2006/main") {
			var p ctypes.Paragraph
			// DecodeElement 会消费至对应结束标签，段落之间天然隔离
			if perr := dec.DecodeElement(&p, &t); perr != nil {
				continue
			}
			writePara(&b, &p)
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

// writePara 拼接段落内全部 run 文本。
func writePara(b *strings.Builder, p *ctypes.Paragraph) {
	if p == nil {
		return
	}
	for _, child := range p.Children {
		if child.Run == nil {
			continue
		}
		for _, rc := range child.Run.Children {
			if rc.Text != nil {
				b.WriteString(rc.Text.Text)
			}
		}
	}
}

func fileExt(filename string) string {
	i := strings.LastIndexByte(filename, '.')
	if i < 0 {
		return ""
	}
	return filename[i:]
}

// unzipEntry 从 zip 字节流中取出指定路径的文件内容；未找到返回空串。
func unzipEntry(data []byte, name string) (string, []byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, err
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", nil, err
		}
		return f.Name, content, nil
	}
	return "", nil, nil
}
