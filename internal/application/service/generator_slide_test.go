package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

func genSection(title string, kps []string) *types.Section {
	kp, _ := json.Marshal(kps)
	return &types.Section{
		ID: types.NewID(), Position: 1, Type: types.SectionTypeSlide, Title: title,
		KnowledgePoints: datatypes.JSON(kp),
	}
}

func genCtxWithLLM(client model.LLMClient) *types.GenerationContext {
	course := &types.Course{Title: "数组入门", Prompt: "学习数组"}
	return &types.GenerationContext{Course: course, Client: client, OutlineSections: []types.OutlineSection{
		{Title: "什么是数组", Type: types.SectionTypeSlide, KnowledgePoints: []string{"定义"}},
	}, DocsText: "参考文档正文"}
}

// 两阶段生成：content 与 steps 均被写入，且 actions 引用元素 id。
func TestSlideGenerateWritesContentAndSteps(t *testing.T) {
	g := &slideGenerator{}
	sec := genSection("数组定义", []string{"同类型元素", "连续存储"})
	client := &seqLLM{contents: []string{testSlideContentJSON, testSlideStepsJSON}}
	err := g.Generate(context.Background(), sec, genCtxWithLLM(client))
	require.NoError(t, err)

	var content types.SlideContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Equal(t, 1280, content.Width)
	require.Equal(t, 720, content.Height)
	require.Len(t, content.Elements, 2)
	require.Equal(t, "e1", content.Elements[0].ID)

	var steps []types.SlideStep
	require.NoError(t, json.Unmarshal(sec.Steps, &steps))
	require.Len(t, steps, 2)
	require.Equal(t, "highlight", steps[0].Actions[0].Type)
	require.Equal(t, "e2", steps[1].Actions[0].TargetElementID)
}

// 非法 targetElementId 的 action 被剔除，但保留文本与合法动作。
func TestSlideGenerateStripsDanglingActions(t *testing.T) {
	g := &slideGenerator{}
	sec := genSection("数组定义", nil)
	steps := `[{"text":"讲 e1","actions":[{"type":"highlight","targetElementId":"e1"},{"type":"box","targetElementId":"ghost"}]}]`
	client := &seqLLM{contents: []string{testSlideContentJSON, steps}}
	err := g.Generate(context.Background(), sec, genCtxWithLLM(client))
	require.NoError(t, err)

	var out []types.SlideStep
	require.NoError(t, json.Unmarshal(sec.Steps, &out))
	require.Len(t, out, 1)
	require.Len(t, out[0].Actions, 1, "悬空引用的动作应被剔除")
	require.Equal(t, "e1", out[0].Actions[0].TargetElementID)
}

// 内容阶段解析失败时自动重试一次，第二次成功。
func TestSlideGenerateRetriesContentOnce(t *testing.T) {
	g := &slideGenerator{}
	sec := genSection("数组定义", nil)
	// 第一次返回非法 JSON，第二次返回合法 content。
	client := &seqLLM{contents: []string{"not json", testSlideContentJSON, testSlideStepsJSON}}
	err := g.Generate(context.Background(), sec, genCtxWithLLM(client))
	require.NoError(t, err)
	require.NotEmpty(t, sec.Content)
	require.NotEmpty(t, sec.Steps)
}

// 连续两次失败返回 error。
func TestSlideGenerateFailsAfterRetries(t *testing.T) {
	g := &slideGenerator{}
	sec := genSection("数组定义", nil)
	client := &seqLLM{contents: []string{"bad", "also bad", testSlideStepsJSON}, err: errors.New("boom")}
	err := g.Generate(context.Background(), sec, genCtxWithLLM(client))
	require.Error(t, err)
	require.Empty(t, sec.Content)
}

// 无 LLM 客户端直接报错。
func TestSlideGenerateRequiresClient(t *testing.T) {
	g := &slideGenerator{}
	err := g.Generate(context.Background(), genSection("x", nil), &types.GenerationContext{})
	require.Error(t, err)
}

// 上下文携带前序已完成环节，供轻量连贯。
func TestSlideSectionContextListsDoneAndCurrent(t *testing.T) {
	genCtx := &types.GenerationContext{
		OutlineSections: []types.OutlineSection{
			{Title: "一", Type: types.SectionTypeSlide, KnowledgePoints: []string{"a"}},
			{Title: "二", Type: types.SectionTypeSlide, KnowledgePoints: []string{"b"}},
		},
		Done: []types.Section{{Position: 1, Title: "一", Type: types.SectionTypeSlide}},
	}
	ctx := slideSectionContext(genCtx, 2)
	require.Contains(t, ctx, "【已完成】")
	require.Contains(t, ctx, "【本环节，正在生成】")
	require.True(t, strings.HasPrefix(ctx, "课程环节"))
}

// ---- 图片生成 ----

// fakeImageClient 返回固定字节或错误，用于 image 元素生成测试。
type fakeImageClient struct {
	data  []byte
	err   error
	calls int
}

func (f *fakeImageClient) GenerateImage(_ context.Context, _ model.ImageRequest) (*model.ImageResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &model.ImageResponse{Data: f.data}, nil
}

// genImageCtx 构造含 image 客户端与存储的上下文。
func genImageCtx(img model.ImageClient, llm model.LLMClient) *types.GenerationContext {
	gc := genCtxWithLLM(llm)
	gc.ImageClient = img
	gc.Storage = &mockStorage{}
	return gc
}

const testSlideContentWithImageJSON = `{"width":1280,"height":720,"background":"#ffffff","accent":"#14b8a6","elements":[{"id":"e1","type":"text","content":"内存布局"},{"id":"e2","type":"image","x":700,"y":160,"width":480,"height":360,"prompt":"A labeled diagram of an array in memory"}]}`
const testSlideStepsSimpleJSON = `[{"text":"讲解一下。","actions":[{"type":"box","targetElementId":"e2"}]}]`

// 有图片客户端时，image 元素生成并回填 src，其余元素不受影响。
func TestSlideGenerateBackfillsImageSrc(t *testing.T) {
	g := &slideGenerator{}
	sec := genSection("内存布局", nil)
	img := &fakeImageClient{data: []byte("png-bytes")}
	client := &seqLLM{contents: []string{testSlideContentWithImageJSON, testSlideStepsSimpleJSON}}
	err := g.Generate(context.Background(), sec, genImageCtx(img, client))
	require.NoError(t, err)
	require.GreaterOrEqual(t, img.calls, 1)

	var content types.SlideContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Len(t, content.Elements, 2)
	require.True(t, strings.HasPrefix(content.Elements[1].Src, "/uploads/"), "image 元素应回填 src")

	var steps []types.SlideStep
	require.NoError(t, json.Unmarshal(sec.Steps, &steps))
	require.Len(t, steps, 1)
	require.Equal(t, "e2", steps[0].Actions[0].TargetElementID)
}

// 未配置图片客户端时，image 元素被剔除，文本等元素保留。
func TestSlideGenerateDropsImageWhenNoClient(t *testing.T) {
	g := &slideGenerator{}
	sec := genSection("内存布局", nil)
	client := &seqLLM{contents: []string{testSlideContentWithImageJSON, testSlideStepsSimpleJSON}}
	err := g.Generate(context.Background(), sec, genCtxWithLLM(client))
	require.NoError(t, err)

	var content types.SlideContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Len(t, content.Elements, 1)
	require.Equal(t, "text", content.Elements[0].Type)
}

// 图片生成失败（重试后仍失败）时剔除该元素，不阻断流程。
func TestSlideGenerateDropsFailedImage(t *testing.T) {
	g := &slideGenerator{}
	sec := genSection("内存布局", nil)
	img := &fakeImageClient{err: errors.New("boom")}
	client := &seqLLM{contents: []string{testSlideContentWithImageJSON, testSlideStepsSimpleJSON}}
	err := g.Generate(context.Background(), sec, genImageCtx(img, client))
	require.NoError(t, err)

	var content types.SlideContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Len(t, content.Elements, 1)
	require.Equal(t, "text", content.Elements[0].Type)
}

// 图片元素缺省 prompt（无 src）时在校验阶段被剔除。
func TestSlideSanitizeRejectsImageWithoutPrompt(t *testing.T) {
	c := &types.SlideContent{Elements: []types.SlideElement{
		{ID: "e1", Type: types.SlideElementText, Content: "标题"},
		{ID: "e2", Type: types.SlideElementImage, X: 10, Y: 10},
	}}
	require.NoError(t, validateSlideContent(c))
	require.Len(t, c.Elements, 1)
}
