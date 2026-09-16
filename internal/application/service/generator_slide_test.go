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
	return &types.GenerationContext{Course: course, Client: client, Retry: types.RetryPolicy{MaxAttempts: 2}, OutlineSections: []types.OutlineSection{
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
const testSlideContentWithMermaidJSON = `{"width":1280,"height":720,"background":"#ffffff","accent":"#14b8a6","elements":[{"id":"e1","type":"mermaid","x":80,"y":160,"width":560,"content":"flowchart TD\n    A[开始] --> B{判断}\n    B -->|是| C[结束]"}]}`
const testSlideMermaidStepsJSON = `[{"text":"跟着流程图走一遍。","actions":[{"type":"box","targetElementId":"e1"}]}]`
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

// mermaid 元素合法时保留；源码为空时被剔除。
func TestSlideSanitizeMermaidElement(t *testing.T) {
	c := &types.SlideContent{Elements: []types.SlideElement{
		{ID: "e1", Type: types.SlideElementText, Content: "流程"},
		{ID: "e2", Type: types.SlideElementMermaid, X: 80, Y: 200, Width: 560,
			Content: "flowchart TD\n    A[开始] --> B[结束]"},
		{ID: "e3", Type: types.SlideElementMermaid, X: 80, Y: 400, Width: 560},
	}}
	require.NoError(t, validateSlideContent(c))
	require.Len(t, c.Elements, 2)
	require.Equal(t, "e1", c.Elements[0].ID)
	require.Equal(t, "e2", c.Elements[1].ID)
}

// 两阶段生成完整透传 mermaid 元素，讲解动作可引用其 id。
func TestSlideGenerateKeepsMermaidElement(t *testing.T) {
	g := &slideGenerator{}
	sec := genSection("排序流程", nil)
	client := &seqLLM{contents: []string{testSlideContentWithMermaidJSON, testSlideMermaidStepsJSON}}
	err := g.Generate(context.Background(), sec, genCtxWithLLM(client))
	require.NoError(t, err)

	var content types.SlideContent
	require.NoError(t, json.Unmarshal(sec.Content, &content))
	require.Len(t, content.Elements, 1)
	require.Equal(t, types.SlideElementMermaid, content.Elements[0].Type)
	require.Contains(t, content.Elements[0].Content, "flowchart TD")

	var steps []types.SlideStep
	require.NoError(t, json.Unmarshal(sec.Steps, &steps))
	require.Len(t, steps, 1)
	require.Equal(t, "e1", steps[0].Actions[0].TargetElementID)
}

// chart 元素合法时保留并补默认尺寸；非法枚举 / 数据长度不对齐 / 空数据被剔除；
// pie 多余系列被丢弃。
func TestSlideSanitizeChartElement(t *testing.T) {
	c := &types.SlideContent{Elements: []types.SlideElement{
		{ID: "e1", Type: types.SlideElementChart, X: 80, Y: 200,
			Chart: types.SlideChartBar, Title: "对比",
			Categories: []string{"Q1", "Q2"},
			Series:     []types.SlideChartSeries{{Name: "线上", Values: []float64{1, 2}}},
		},
		{ID: "e2", Type: types.SlideElementChart, X: 80, Y: 200,
			Chart:      "donut", // 非法枚举
			Categories: []string{"A"}, Series: []types.SlideChartSeries{{Name: "x", Values: []float64{1}}},
		},
		{ID: "e3", Type: types.SlideElementChart, X: 80, Y: 200,
			Chart:      types.SlideChartLine,
			Categories: []string{"A", "B"},
			Series:     []types.SlideChartSeries{{Name: "x", Values: []float64{1}}}, // 长度不对齐
		},
		{ID: "e4", Type: types.SlideElementChart, X: 80, Y: 200,
			Chart:      types.SlideChartPie,
			Categories: []string{"A", "B"},
			Series: []types.SlideChartSeries{
				{Name: "x", Values: []float64{1, 2}},
				{Name: "y", Values: []float64{3, 4}}, // pie 多余系列
			},
		},
	}}
	require.NoError(t, validateSlideContent(c))
	require.Len(t, c.Elements, 2)

	bar := c.Elements[0]
	require.Equal(t, 560, bar.Width)
	require.Equal(t, 360, bar.Height)
	require.Len(t, bar.Series, 1)

	pie := c.Elements[1]
	require.Equal(t, types.SlideElementChart, pie.Type)
	require.Len(t, pie.Series, 1)
	require.Equal(t, "x", pie.Series[0].Name)
}

// ---- functionPlot 元素校验 ----

// 合法表达式保留并补默认尺寸；非法窗口回退默认；非法表达式 / 无曲线的元素被剔除；
// 曲线上限 4 条，非法色被清空。
func TestSlideSanitizeFunctionPlotElement(t *testing.T) {
	c := &types.SlideContent{Elements: []types.SlideElement{
		{ID: "e1", Type: types.SlideElementFunctionPlot, X: 80, Y: 160,
			XRange: [2]float64{-6.5, 6.5},
			Curves: []types.SlideFunctionPlotCurve{
				{Expression: "sin(x)", Color: "blue"}, // 非法色清空
				{Expression: "2*sin(x)-1", Dash: true},
			},
		},
		{ID: "e2", Type: types.SlideElementFunctionPlot, X: 80, Y: 160}, // 无曲线
		{ID: "e3", Type: types.SlideElementFunctionPlot, X: 80, Y: 160,
			Curves: []types.SlideFunctionPlotCurve{
				{Expression: "sin(x)"}, {Expression: "cos(x)"},
				{Expression: "tan(x)"}, {Expression: "sqrt(x)"},
				{Expression: "abs(x)"}, // 第 5 条被丢弃
			},
		},
		{ID: "e4", Type: types.SlideElementFunctionPlot, X: 80, Y: 160,
			Curves: []types.SlideFunctionPlotCurve{
				{Expression: "x.value();"}, // 非法字符
				{Expression: "hack(x)"},    // 未知函数名
			},
		},
	}}
	require.NoError(t, validateSlideContent(c))
	require.Len(t, c.Elements, 2)

	plot := c.Elements[0]
	require.Equal(t, 560, plot.Width)
	require.Equal(t, 360, plot.Height)
	require.Empty(t, plot.Curves[0].Color)
	require.True(t, validFunctionRange(plot.XRange))
	tp := c.Elements[1]
	require.Equal(t, types.SlideElementFunctionPlot, tp.Type)
	require.Len(t, tp.Curves, 4)
}

// 非法 xRange 回退默认 [-6,6]；非法 yRange 归零（前端自适应）。
func TestSlideSanitizeFunctionPlotRange(t *testing.T) {
	el := &types.SlideElement{ID: "e1", Type: types.SlideElementFunctionPlot, X: 0, Y: 0,
		XRange: [2]float64{5, -5},
		YRange: [2]float64{3, 3},
		Curves: []types.SlideFunctionPlotCurve{{Expression: "x^2"}},
	}
	require.True(t, sanitizeFunctionPlotElement(el))
	require.Equal(t, [2]float64{-6, 6}, el.XRange)
	require.Equal(t, [2]float64{}, el.YRange)

	el.YRange = [2]float64{-2, 2}
	require.True(t, sanitizeFunctionPlotElement(el))
	require.Equal(t, [2]float64{-2, 2}, el.YRange)
}

// 表达式白名单：合法样本全通过，非法样本全拒绝。
func TestValidFunctionExpression(t *testing.T) {
	valid := []string{
		"x", "2*x+1", "-3x^2", "sin(x)", "2sin(x)+cos(2x)", "sqrt(abs(x))",
		"exp(-x/2)*cos(pi*x)", "log(x)", "x^2/4 - 1", "e^(-x^2/2)",
	}
	for _, v := range valid {
		require.True(t, validFunctionExpression(v), v)
	}
	invalid := []string{
		"", "   ", "sin(1).value", "eval('x')", "fetch(x)", "x;", "a_variable",
		"function(){return 1}", "import x", strings.Repeat("x", 201),
	}
	for _, s := range invalid {
		require.False(t, validFunctionExpression(s), s)
	}
}

// ---- 白板类动作校验 ----

// oidSet 构造元素 id 集合，供 sanitizeSlideSteps 测试使用。
func oidSet(ids ...string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// laser：元素引用或坐标至少其一；两者皆无则剔除；坐标被 clamp 进画布。
func TestSlideSanitizeStepLaser(t *testing.T) {
	in := []types.SlideStep{{
		Text: "指向重点",
		Actions: []types.SlideAction{
			{Type: types.SlideActionLaser, TargetElementID: "e1"},
			{Type: types.SlideActionLaser, X: 1300, Y: -10},
			{Type: types.SlideActionLaser, TargetElementID: "ghost"},
			{Type: types.SlideActionLaser},
		},
	}}
	out := sanitizeSlideSteps(in, oidSet("e1"), 1280, 720)
	require.Len(t, out[0].Actions, 2)
	require.Equal(t, "e1", out[0].Actions[0].TargetElementID)
	require.Empty(t, out[0].Actions[1].TargetElementID)
	require.Equal(t, float64(1280), out[0].Actions[1].X)
	require.Equal(t, float64(0), out[0].Actions[1].Y)
}

// draw：合法笔画保留并补缺省粗细；非法 kind / 空点集 / 空 text 被剔除；
// pen 点集截断到 64 点、坐标 clamp 进画布。
func TestSlideSanitizeStepDraw(t *testing.T) {
	pts := make([][2]float64, 80)
	for i := range pts {
		pts[i] = [2]float64{float64(i), float64(i) * 10}
	}
	in := []types.SlideStep{{
		Text: "画个圈",
		Actions: []types.SlideAction{
			{Type: types.SlideActionDraw, Drawing: &types.SlideDrawing{
				Kind: types.SlideDrawPen, Points: pts,
			}},
			{Type: types.SlideActionDraw, Drawing: &types.SlideDrawing{Kind: "spray"}},            // 非法 kind
			{Type: types.SlideActionDraw, Drawing: &types.SlideDrawing{Kind: types.SlideDrawPen}}, // 无点集
			{Type: types.SlideActionDraw, Drawing: &types.SlideDrawing{
				Kind: types.SlideDrawText, Content: " ", // 空 text
			}},
			{Type: types.SlideActionDraw, Drawing: &types.SlideDrawing{
				Kind: types.SlideDrawRect, X: 100, Y: 100,
			}}, // 无宽高
			{Type: types.SlideActionDraw, Drawing: nil}, // 无笔画数据
		},
	}}
	out := sanitizeSlideSteps(in, nil, 1280, 720)
	require.Len(t, out[0].Actions, 1)
	d := out[0].Actions[0].Drawing
	require.Len(t, d.Points, 64)
	require.Equal(t, float64(63), d.Points[63][0])
	require.Equal(t, float64(630), d.Points[63][1])
	require.Equal(t, types.SlideDrawSizeMedium, d.Size, "粗细缺省 medium")

	// 坐标 clamp 进画布
	in2 := []types.SlideStep{{
		Text: "画一笔",
		Actions: []types.SlideAction{
			{Type: types.SlideActionDraw, Drawing: &types.SlideDrawing{
				Kind:   types.SlideDrawPen,
				Points: [][2]float64{{-5, -5}, {1300, 800}},
			}},
		},
	}}
	out2 := sanitizeSlideSteps(in2, nil, 1280, 720)
	require.Len(t, out2[0].Actions, 1)
	pts2 := out2[0].Actions[0].Drawing.Points
	require.Equal(t, [2]float64{0, 0}, pts2[0])
	require.Equal(t, [2]float64{1280, 720}, pts2[1])
}

// clearBoard：无附加字段、原样保留。
func TestSlideSanitizeStepClearBoard(t *testing.T) {
	in := []types.SlideStep{{
		Text: "擦掉重新画",
		Actions: []types.SlideAction{
			{Type: types.SlideActionClearBoard, TargetElementID: "e1", X: 1, Y: 2,
				Drawing: &types.SlideDrawing{Kind: types.SlideDrawRect}},
		},
	}}
	out := sanitizeSlideSteps(in, oidSet("e1"), 1280, 720)
	require.Len(t, out[0].Actions, 1)
	a := out[0].Actions[0]
	require.Equal(t, types.SlideActionClearBoard, a.Type)
	require.Empty(t, a.TargetElementID)
	require.Nil(t, a.Drawing)
}
