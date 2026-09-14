package service

import (
	"context"
	"io"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/StellarisJAY/agent-classroom/internal/model/extractor"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// DocBudget 参考文档注入提示词的 token 预算（换算为字符数裁剪）。
type DocBudget struct {
	MaxTokens     int
	CharsPerToken float64
}

// budgetChars 预算对应的字符数上限；MaxTokens<=0 视为不限制。
func (b DocBudget) budgetChars() int {
	if b.MaxTokens <= 0 {
		return math.MaxInt
	}
	r := b.CharsPerToken
	if r <= 0 {
		r = 1.5
	}
	return int(float64(b.MaxTokens) * r)
}

// docLoader 参考文档提取与文本加载的唯一入口：
// - EnsureExtracted：幂等补跑提取并落库缓存（建课异步与生成前置共用）；
// - LoadDocsText：按 token 预算把已提取文本裁剪为单段注入提示词的引用块。
// 提取状态以 document.extracted_status 为唯一事实源，进程内存只做并发去重。
type docLoader struct {
	docRepo   types.DocumentRepo
	storage   types.Storage
	extractor extractor.Extractor
	budget    DocBudget
	// extractTimeout 生成前置同步等待提取的超时（EnsureExtractedWithTimeout）
	extractTimeout time.Duration
	// per-course singleflight：避免异步提取与用户快速触发生成导致重复提取
	mus sync.Map // courseID -> *sync.Mutex
}

// NewDocLoader 创建文档提取/加载器（bootstrap 装配入口）。
func NewDocLoader(docRepo types.DocumentRepo, storage types.Storage, ext extractor.Extractor, budget DocBudget, extractTimeout time.Duration) *docLoader {
	return newDocLoader(docRepo, storage, ext, budget, extractTimeout)
}

func newDocLoader(
	docRepo types.DocumentRepo,
	storage types.Storage,
	ext extractor.Extractor,
	budget DocBudget,
	extractTimeout time.Duration,
) *docLoader {
	return &docLoader{docRepo: docRepo, storage: storage, extractor: ext, budget: budget, extractTimeout: extractTimeout}
}

// EnsureExtracted 确保课程全部文档的提取状态收敛（success/failed）。
// pending 文档当场提取（幂等）；failed 文档默认跳过（重试需显式 retry=true）。
// 返回 (成功文本数, 失败数)。读取/提取失败不返回错误，仅记录状态与日志。
func (l *docLoader) EnsureExtracted(ctx context.Context, courseID types.ID, retryFailed bool) (ok, failedSoFar int, err error) {
	muAny, _ := l.mus.LoadOrStore(courseID, &sync.Mutex{})
	mu := muAny.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	docs, derr := l.docRepo.ListByCourse(ctx, courseID)
	if derr != nil {
		return 0, 0, derr
	}
	for i := range docs {
		doc := docs[i]
		switch doc.ExtractedStatus {
		case types.ExtractStatusSuccess:
			ok++
			continue
		case types.ExtractStatusFailed:
			if !retryFailed {
				failedSoFar++
				continue
			}
		}
		text, xerr := l.extractDoc(ctx, &doc)
		if xerr != nil {
			slog.Warn("document extraction failed",
				"document_id", doc.ID.String(), "filename", doc.Filename, "error", xerr)
			if uerr := l.docRepo.UpdateExtracted(ctx, doc.ID, types.ExtractStatusFailed, ""); uerr != nil {
				return ok, failedSoFar, uerr
			}
			failedSoFar++
			continue
		}
		if uerr := l.docRepo.UpdateExtracted(ctx, doc.ID, types.ExtractStatusSuccess, text); uerr != nil {
			return ok, failedSoFar, uerr
		}
		ok++
	}
	return ok, failedSoFar, nil
}

// EnsureExtractedWithTimeout 生成前置等待：在超时内幂等等待提取收敛。
// 超时返回 ErrDocExtracting；课程有文档但全部提取失败返回 ErrDocExtractFailed；无文档成功返回 0,0。
func (l *docLoader) EnsureExtractedWithTimeout(ctx context.Context, courseID types.ID) (okCnt, failedCnt int, err error) {
	timeout := l.extractTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	hasDoc := false
	docs, derr := l.docRepo.ListByCourse(ctx, courseID)
	if derr == nil {
		for i := range docs {
			if docs[i].ExtractedStatus != types.ExtractStatusSuccess || docs[i].ExtractedText == nil {
				hasDoc = true
			}
		}
	}
	if !hasDoc {
		return 0, 0, nil
	}

	okCnt, failedCnt, err = l.EnsureExtracted(wctx, courseID, false)
	if err != nil && wctx.Err() != nil {
		return okCnt, failedCnt, types.ErrDocExtracting
	}
	if err != nil {
		return okCnt, failedCnt, err
	}
	if okCnt+failedCnt > 0 && okCnt == 0 {
		return okCnt, failedCnt, types.ErrDocExtractFailed
	}
	return okCnt, failedCnt, nil
}

// extractDoc 从 storage 读文件并提取（单文档内先外部后本地的编排由 chain 决定）。
func (l *docLoader) extractDoc(ctx context.Context, doc *types.Document) (string, error) {
	rc, err := l.storage.Get(ctx, doc.URL)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	// 局部大小限制：超出上限的文件直接判失败，避免读入超大异常数据
	lr := io.LimitReader(rc, util.MaxUploadBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	if len(data) > util.MaxUploadBytes {
		return "", types.ErrFileTooLarge
	}
	res, err := l.extractor.Extract(ctx, doc.Filename, data)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

// LoadDocsText 装配课程全部成功提取文档的引用文本块。
// 预算按字符数（MaxTokens × CharsPerToken）分片：每文档先给均分配额，截断尾部并打标记；
// 未用满配额部分顺延给后续文档（顺序累计）。
func (l *docLoader) LoadDocsText(ctx context.Context, courseID types.ID) (string, error) {
	docs, err := l.docRepo.ListByCourse(ctx, courseID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	remaining := l.budget.budgetChars()
	n := len(docs)
	parts := make([]string, 0, n)
	for i := range docs {
		doc := docs[i]
		if doc.ExtractedStatus != types.ExtractStatusSuccess || doc.ExtractedText == nil {
			continue
		}
		// 未消费文档数（含当前），配额未用满部分顺延给后续文档
		notYet := 0
		for j := i; j < len(docs); j++ {
			if docs[j].ExtractedStatus == types.ExtractStatusSuccess {
				notYet++
			}
		}
		allowance := 0
		if notYet > 0 {
			allowance = remaining / notYet
		}
		text := *doc.ExtractedText
		truncated := false
		if allowance < len(text) {
			text = text[:allowance]
			truncated = true
		}
		remaining -= len(text)
		parts = append(parts, renderDocBlock(doc.Filename, text, truncated))
	}
	for i, p := range parts {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(p)
	}
	return b.String(), nil
}

// renderDocBlock 单篇文档的引用块：文件名分隔头 + 正文 + 截断标记。
func renderDocBlock(filename, text string, truncated bool) string {
	var b strings.Builder
	b.WriteString("\n===== 文档: ")
	b.WriteString(filename)
	if truncated {
		b.WriteString("（超出预算，已截断）")
	}
	b.WriteString(" =====\n")
	b.WriteString(text)
	if truncated {
		b.WriteString("\n[...该文档因超出长度预算被截断，未展示部分请以主题理解为准...]")
	}
	return b.String()
}
