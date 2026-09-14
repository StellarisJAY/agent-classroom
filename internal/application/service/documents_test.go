package service

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/gomutex/godocx"
	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/model/extractor"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// fakeDocRepo 提取与裁剪测试用 document repo，读写在内存中完成。
type fakeDocRepo struct {
	docs []types.Document
}

var _ types.DocumentRepo = (*fakeDocRepo)(nil)

func (f *fakeDocRepo) Create(_ context.Context, d *types.Document) error {
	f.docs = append(f.docs, *d)
	return nil
}
func (f *fakeDocRepo) ListByCourse(_ context.Context, _ types.ID) ([]types.Document, error) {
	return f.docs, nil
}
func (f *fakeDocRepo) UpdateExtracted(_ context.Context, id types.ID, status, text string) error {
	for i := range f.docs {
		if f.docs[i].ID == id {
			f.docs[i].ExtractedStatus = status
			f.docs[i].ExtractedText = &text
			return nil
		}
	}
	return types.ErrNotFound
}

type roStorage struct{ url string; data []byte }

func (s roStorage) Put(_ context.Context, key string, _ io.Reader) (string, error) { return s.url, nil }
func (s roStorage) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(s.data))), nil
}

// loaderFor 按文档构造假文档集 + 预算/loader。
func loaderFor(docs []types.Document, budget DocBudget) (*docLoader, *fakeDocRepo) {
	repo := &fakeDocRepo{docs: docs}
	storage := roStorage{}
	return newDocLoader(repo, storage, extractor.NewChain(extractor.NewLocal(), nil), budget, 0), nil
}

func TestDocLoaderEnsureExtractedIdempotent(t *testing.T) {
	id := types.NewID()
	text := "参考文档正文"
	repo := &fakeDocRepo{docs: []types.Document{
		{ID: id, Filename: "a.txt", URL: "u", ExtractedStatus: types.ExtractStatusPending},
		{ID: types.NewID(), Filename: "b.txt", URL: "u", ExtractedStatus: types.ExtractStatusSuccess, ExtractedText: &text},
	}}
	l := newDocLoader(repo, roStorage{data: []byte("参考文档正文")}, extractor.NewChain(extractor.NewLocal(), nil), DocBudget{MaxTokens: 1000}, 0)

	ok, failed, err := l.EnsureExtracted(context.Background(), id, false)
	require.NoError(t, err)
	require.Equal(t, 2, ok)
	require.Equal(t, 0, failed)
	docs, _ := repo.ListByCourse(context.Background(), id)
	require.Equal(t, types.ExtractStatusSuccess, docs[0].ExtractedStatus)
	require.Equal(t, "参考文档正文", *docs[0].ExtractedText)

	// 再次执行：幂等，不再重复提取
	ok2, failed2, err := l.EnsureExtracted(context.Background(), id, false)
	require.NoError(t, err)
	require.Equal(t, 2, ok2)
	require.Equal(t, 0, failed2)
}

func TestDocLoaderLoadDocsTextBudget(t *testing.T) {
	// 两篇各 200 字符文档，预算 30 字符：每篇均分配额 15，均截断
	text200 := strings.Repeat("字", 200)
	docs := []types.Document{
		{ID: types.NewID(), Filename: "a.md", ExtractedStatus: types.ExtractStatusSuccess, ExtractedText: &text200},
		{ID: types.NewID(), Filename: "b.md", ExtractedStatus: types.ExtractStatusSuccess, ExtractedText: &text200},
	}
	l, _ := loaderFor(docs, DocBudget{MaxTokens: 30, CharsPerToken: 1})
	out, err := l.LoadDocsText(context.Background(), types.NewID())
	require.NoError(t, err)
	require.Contains(t, out, "===== 文档: a.md（超出预算，已截断）")
	require.Contains(t, out, "超出长度预算被截断")
	require.Equal(t, 2, strings.Count(out, "超出预算，已截断"))
}

func TestDocLoaderLoadDocsTextCarryover(t *testing.T) {
	// 第一篇 10 字符，配额 20：剩余 10 顺延给第二篇，即第二篇可拿到 40-10=30 中的更多
	short := "0123456789"
	long := strings.Repeat("L", 100)
	docs := []types.Document{
		{ID: types.NewID(), Filename: "s.md", ExtractedStatus: types.ExtractStatusSuccess, ExtractedText: &short},
		{ID: types.NewID(), Filename: "l.md", ExtractedStatus: types.ExtractStatusSuccess, ExtractedText: &long},
	}
	l, _ := loaderFor(docs, DocBudget{MaxTokens: 40, CharsPerToken: 1})
	out, err := l.LoadDocsText(context.Background(), types.NewID())
	require.NoError(t, err)
	// 第二篇截断至 40-10=30 字符
	require.Contains(t, out, strings.Repeat("L", 30))
	require.NotContains(t, out, strings.Repeat("L", 31))
}

func TestDocLoaderNoBudgetNoTruncate(t *testing.T) {
	long := strings.Repeat("L", 100)
	docs := []types.Document{
		{ID: types.NewID(), Filename: "a.md", ExtractedStatus: types.ExtractStatusSuccess, ExtractedText: &long},
	}
	l, _ := loaderFor(docs, DocBudget{MaxTokens: 0})
	out, err := l.LoadDocsText(context.Background(), types.NewID())
	require.NoError(t, err)
	require.Contains(t, out, strings.Repeat("L", 100))
	require.NotContains(t, out, "已截断")
}

func TestExtractLocalDocx(t *testing.T) {
	// 用 godocx 生成一个含两段落的 docx，再走本地提取器
	root, err := godocx.NewDocument()
	require.NoError(t, err)
	root.AddParagraph("第一段：数组与循环")
	root.AddParagraph("第二段：条件分支")
	var buf bytes.Buffer
	_, err = root.WriteTo(&buf)
	require.NoError(t, err)

	res, err := extractor.NewLocal().Extract(context.Background(), "notes.docx", buf.Bytes())
	require.NoError(t, err)
	require.Contains(t, res.Text, "第一段：数组与循环")
	require.Contains(t, res.Text, "第二段：条件分支")
}

func TestChainFallsBackToLocal(t *testing.T) {
	// minerU 未配置（nil）→ pdf/docx 直接本地；本地提取空仍返回成功，上游按空文本处理
	res, err := extractor.NewChain(extractor.NewLocal(), nil).Extract(
		context.Background(), "a.md", []byte("markdown 文本"))
	require.NoError(t, err)
	require.Equal(t, "local", res.Source)
	require.Equal(t, "markdown 文本", res.Text)

	_, err = extractor.NewChain(extractor.NewLocal(), nil).Extract(
		context.Background(), "bad.pdf", []byte("not a pdf"))
	require.Error(t, err)
}
