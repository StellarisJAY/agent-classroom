package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// captureQuestionRepo 记录整表替换写入的题目。
type captureQuestionRepo struct {
	mu       sync.Mutex
	replaced []types.Question
}

func (c *captureQuestionRepo) ReplaceBySection(_ context.Context, _ types.ID, qs []types.Question) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replaced = append([]types.Question(nil), qs...)
	return nil
}
func (c *captureQuestionRepo) ListBySection(context.Context, types.ID) ([]types.Question, error) {
	return nil, nil
}

func quizGenCtx(client model.LLMClient) *types.GenerationContext {
	return &types.GenerationContext{
		Course:   &types.Course{Title: "数组入门", Prompt: "学习数组", OwnerID: types.NewID()},
		Client:   client,
		Thinking: model.ThinkingOff,
		DocsText: "参考文档正文",
		Retry:    types.RetryPolicy{MaxAttempts: 2},
	}
}

// 正常单阶段生成：题目被写入 questionRepo，position 从 1 递增。
func TestQuizGenerateWritesQuestions(t *testing.T) {
	repo := &captureQuestionRepo{}
	g := &quizGenerator{questionRepo: repo}
	sec := genSection("数组基础", []string{"下标", "连续存储"})
	sec.Type = types.SectionTypeQuiz
	client := &seqLLM{contents: []string{testQuizQuestionsJSON}}
	require.NoError(t, g.Generate(context.Background(), sec, quizGenCtx(client)))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.replaced, 2)
	q1 := repo.replaced[0]
	require.Equal(t, types.QuestionTypeSingle, q1.Type)
	require.Equal(t, 1, q1.Position)
	require.Equal(t, "数组下标从几开始？", q1.Stem)
	var opts []string
	require.NoError(t, json.Unmarshal(q1.Options, &opts))
	require.Equal(t, []string{"0", "1"}, opts)
	var ans []int
	require.NoError(t, json.Unmarshal(q1.Answers, &ans))
	require.Equal(t, []int{0}, ans)
	require.NotNil(t, q1.CreateBy)
}

// 非法题目被剔除（非法类型 / single 多答案 / 解析缺项），合法题保留。
func TestQuizGenerateSanitizesInvalid(t *testing.T) {
	repo := &captureQuestionRepo{}
	g := &quizGenerator{questionRepo: repo}
	sec := genSection("x", nil)
	raw := `[
	  {"type":"single","stem":"a","options":["1","2","3"],"answers":[0],"explanations":["","",""]},
	  {"type":"single","stem":"ok 单选","options":["0","1","2"],"answers":[0],"explanations":["对","错","错"]},
	  {"type":"single","stem":"single 多答案非法","options":["0","1","2"],"answers":[0,1],"explanations":["对","对","错"]},
	  {"type":"single","stem":"下标越界","options":["0","1"],"answers":[5],"explanations":["对","错"]},
	  {"type":"single","stem":"缺解析","options":["0","1"],"answers":[0],"explanations":["对"]},
	  {"type":"badtype","stem":"类型非法","options":["0","1","2"],"answers":[0],"explanations":["对","错","错"]},
	  {"type":"multiple","stem":"多选合法","options":["a","b","c","d"],"answers":[0,2],"explanations":["对","错","对","错"]}
	]`
	client := &seqLLM{contents: []string{raw}}
	require.NoError(t, g.Generate(context.Background(), sec, quizGenCtx(client)))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.replaced, 2, "应只保留两题合法题")
	require.Equal(t, "ok 单选", repo.replaced[0].Stem)
	require.Equal(t, "多选合法", repo.replaced[1].Stem)
}

// 解析失败自动重试一次，第二次成功。
func TestQuizGenerateRetriesOnce(t *testing.T) {
	repo := &captureQuestionRepo{}
	g := &quizGenerator{questionRepo: repo}
	sec := genSection("x", nil)
	client := &seqLLM{contents: []string{"not json", testQuizQuestionsJSON}}
	require.NoError(t, g.Generate(context.Background(), sec, quizGenCtx(client)))
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.replaced, 2)
}

// 两次失败返回 error，不写库。
func TestQuizGenerateFailsAfterRetries(t *testing.T) {
	repo := &captureQuestionRepo{}
	g := &quizGenerator{questionRepo: repo}
	sec := genSection("x", nil)
	client := &seqLLM{contents: []string{"bad", "bad"}, err: errors.New("boom")}
	require.Error(t, g.Generate(context.Background(), sec, quizGenCtx(client)))
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Empty(t, repo.replaced)
}

// 缺少依赖直接报错。
func TestQuizGenerateRequiresDeps(t *testing.T) {
	sec := genSection("x", nil)
	require.Error(t, (&quizGenerator{}).Generate(context.Background(), sec, quizGenCtx(&seqLLM{})))
	require.Error(t, (&quizGenerator{questionRepo: &captureQuestionRepo{}}).Generate(context.Background(), sec, &types.GenerationContext{}))
}

// 全部题目非法时判失败并重试。
func TestQuizGenerateAllInvalidRetries(t *testing.T) {
	repo := &captureQuestionRepo{}
	g := &quizGenerator{questionRepo: repo}
	sec := genSection("x", nil)
	// 第一次全非法，第二次合法。
	client := &seqLLM{contents: []string{`[{"type":"single","stem":"","options":["0","1"],"answers":[0],"explanations":["",""]}]`, testQuizQuestionsJSON}}
	require.NoError(t, g.Generate(context.Background(), sec, quizGenCtx(client)))
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.replaced, 2)
}
