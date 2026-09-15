package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"text/template"

	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// Slide 环节的两阶段生成：阶段一产出视觉内容（section.content），
// 阶段二参考元素产出讲解步骤（section.steps）。见 docs/slide数据结构.md。
// 各阶段 LLM 解析/校验失败自动重试一次（共 2 次尝试）。
// 不设 max_tokens：思考模式下 reasoning_content 会占用大量 token，故交模型默认上限。

// slideGenerator Slide 环节内容生成器。模型客户端与上下文由 SectionService
// 通过 GenerationContext 注入；无额外持久依赖。
type slideGenerator struct{}

// slideUserData Slide 阶段一 user 模板填充字段。
type slideUserData struct {
	CourseTitle      string
	SectionTitle     string
	KnowledgePoints  string
	Prompt           string
	PreviousSections string
	DocsSummary      string
	UserRequest      string
}

// slideStepsUserData Slide 阶段二 user 模板填充字段。
type slideStepsUserData struct {
	CourseTitle      string
	SectionTitle     string
	KnowledgePoints  string
	Prompt           string
	PreviousSections string
	ElementSummary   string
}

// Generate 两阶段生成并写回 section.Content / section.Steps。
func (g *slideGenerator) Generate(ctx context.Context, section *types.Section, genCtx *types.GenerationContext) error {
	if genCtx == nil || genCtx.Client == nil {
		return errors.New("slide generator: missing llm client")
	}

	prev := slideSectionContext(genCtx, section.Position)

	// 阶段一：生成视觉内容。
	content, err := g.generateContent(ctx, section, genCtx, prev)
	if err != nil {
		return err
	}
	// 图片生成环节：为 image 元素生成图片并回填 src（失败元素剔除）。
	g.generateImages(ctx, section, genCtx, content)
	// 阶段二：参考元素生成讲解步骤。
	steps, err := g.generateSteps(ctx, section, genCtx, prev, content)
	if err != nil {
		return err
	}

	cJSON, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("marshal slide content: %w", err)
	}
	sJSON, err := json.Marshal(steps)
	if err != nil {
		return fmt.Errorf("marshal slide steps: %w", err)
	}
	section.Content = datatypes.JSON(cJSON)
	section.Steps = datatypes.JSON(sJSON)
	return nil
}

// generateContent 阶段一：产出并校验视觉内容。
func (g *slideGenerator) generateContent(ctx context.Context, section *types.Section, genCtx *types.GenerationContext, prev string) (*types.SlideContent, error) {
	msgData := slideUserData{
		CourseTitle:      courseTitle(genCtx),
		SectionTitle:     section.Title,
		KnowledgePoints:  sectionKnowledgePoints(section),
		Prompt:           sectionPrompt(section),
		PreviousSections: prev,
		DocsSummary:      strings.TrimSpace(genCtx.DocsText),
		UserRequest:      genCtxUserRequest(genCtx),
	}
	messages, err := renderSlideMessages(slideUserTpl, msgData, slideSystemPrompt)
	if err != nil {
		return nil, err
	}

	var content *types.SlideContent
	err = retryCall(func() error {
		slog.Debug("generating slide content")
		resp, cerr := chatOnce(ctx, genCtx.Client, messages, 0.3, genCtx.Thinking)
		if cerr != nil {
			return cerr
		}
		var out types.SlideContent
		if xerr := util.ExtractJSON(resp.Content, &out); xerr != nil {
			slog.Warn("invalid slide content", "content", resp.Content)
			return fmt.Errorf("parse slide content json: %w", xerr)
		}
		content = &out
		slog.Debug("generate slide content done")
		return validateSlideContent(&out)
	})
	if err != nil {
		slog.Warn("slide content generation failed", "section_id", section.ID.String(), "error", err)
		return nil, err
	}
	return content, nil
}

// generateSteps 阶段二：参考元素产出并校验讲解步骤。
func (g *slideGenerator) generateSteps(ctx context.Context, section *types.Section, genCtx *types.GenerationContext, prev string, content *types.SlideContent) ([]types.SlideStep, error) {
	msgData := slideStepsUserData{
		CourseTitle:      courseTitle(genCtx),
		SectionTitle:     section.Title,
		KnowledgePoints:  sectionKnowledgePoints(section),
		Prompt:           sectionPrompt(section),
		PreviousSections: prev,
		ElementSummary:   slideElementSummary(content),
	}
	messages, err := renderSlideMessages(slideStepsUserTpl, msgData, slideStepsSystemPrompt)
	if err != nil {
		return nil, err
	}

	idSet := elementIDSet(content)
	var steps []types.SlideStep
	err = retryCall(func() error {
		slog.Debug("generating slide steps")
		resp, cerr := chatOnce(ctx, genCtx.Client, messages, 0.5, genCtx.Thinking)
		if cerr != nil {
			return cerr
		}
		var out []types.SlideStep
		if xerr := util.ExtractJSON(resp.Content, &out); xerr != nil {
			slog.Warn("invalid slide content", "content", resp.Content)
			return fmt.Errorf("parse slide steps json: %w", xerr)
		}
		out = sanitizeSlideSteps(out, idSet, content.Width, content.Height)
		if len(out) == 0 {
			return errors.New("slide steps: empty narration after sanitize")
		}
		steps = out
		slog.Debug("generate slide steps done")
		return nil
	})
	if err != nil {
		slog.Warn("slide steps generation failed", "section_id", section.ID.String(), "error", err)
		return nil, err
	}
	return steps, nil
}

// ---- 图片生成 ----

// generateImages 图片生成环节：为 content 中 src 为空的 image 元素调用文生图模型，
// 写入对象存储并回填 src；单图重试后仍失败则删除该元素，不阻断流程。
// 未配置图片模型或存储时移除全部 image 元素（此时提示词亦要求不使用图片）。
func (g *slideGenerator) generateImages(ctx context.Context, section *types.Section, genCtx *types.GenerationContext, content *types.SlideContent) {
	if genCtx.ImageClient == nil || genCtx.Storage == nil {
		content.Elements = dropImageElements(content.Elements)
		return
	}
	out := make([]types.SlideElement, 0, len(content.Elements))
	for _, el := range content.Elements {
		if el.Type != types.SlideElementImage || el.Src != "" {
			out = append(out, el)
			continue
		}
		slog.Debug("generating image element", "prompt", el.Prompt)
		url, err := generateOneImage(ctx, section, genCtx, el)
		if err != nil {
			slog.Warn("slide image generation failed, dropping element",
				"section_id", section.ID.String(), "element_id", el.ID, "error", err)
			continue
		}
		el.Src = url
		out = append(out, el)
	}
	content.Elements = out
}

// generateOneImage 生成单张图片并写入对象存储，返回可访问 url。
// 优先使用与元素宽高比匹配的尺寸；该尺寸失败时回退默认正方形尺寸再试一次。
func generateOneImage(ctx context.Context, section *types.Section, genCtx *types.GenerationContext, el types.SlideElement) (string, error) {
	sizes := imageSizeCandidates(el.Width, el.Height)
	var url string
	var lastErr error
	for _, size := range sizes {
		var u string
		err := retryCall(func() error {
			resp, gerr := genCtx.ImageClient.GenerateImage(ctx, model.ImageRequest{Prompt: el.Prompt, Size: size})
			if gerr != nil {
				return gerr
			}
			if len(resp.Data) == 0 {
				return errors.New("slide image: empty image data")
			}
			key := fmt.Sprintf("courses/%s/sections/%s/%s.png", section.CourseID.String(), section.ID.String(), el.ID)
			stored, perr := genCtx.Storage.Put(ctx, key, bytes.NewReader(resp.Data))
			if perr != nil {
				return perr
			}
			u = stored
			return nil
		})
		if err == nil {
			url = u
			return url, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// defaultImageSize 文生图默认尺寸。
const defaultImageSize = "1024x1024"

// imageSizeCandidates 按元素宽高比返回候选生成尺寸（首个优先，末位为默认兜底）。
func imageSizeCandidates(width, height int) []string {
	if width <= 0 || height <= 0 {
		return []string{defaultImageSize}
	}
	ratio := float64(width) / float64(height)
	switch {
	case ratio >= 1.2:
		return []string{"1536x1024", defaultImageSize}
	case ratio <= 0.83:
		return []string{"1024x1536", defaultImageSize}
	default:
		return []string{defaultImageSize}
	}
}

// dropImageElements 移除全部 image 元素（图片模型不可用时调用）。
func dropImageElements(in []types.SlideElement) []types.SlideElement {
	out := make([]types.SlideElement, 0, len(in))
	for _, el := range in {
		if el.Type == types.SlideElementImage {
			continue
		}
		out = append(out, el)
	}
	return out
}

// ---- 渲染与调用 ----

// renderSlideMessages 渲染给定 user 模板并拼接对应 system 提示词。
func renderSlideMessages(tpl *template.Template, data any, system string) ([]model.ChatMessage, error) {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render slide prompt: %w", err)
	}
	return []model.ChatMessage{
		{Role: model.RoleSystem, Content: system},
		{Role: model.RoleUser, Content: buf.String()},
	}, nil
}

// chatOnce 单次调用模型返回完整内容。
func chatOnce(ctx context.Context, client model.LLMClient, messages []model.ChatMessage, temp float64, thinking string) (*model.ChatResponse, error) {
	return client.Chat(ctx, model.ChatRequest{
		Messages:    messages,
		Temperature: &temp,
		Thinking:    thinking,
	})
}

// retryCall 执行 fn，失败重试一次（共最多 2 次尝试）。
func retryCall(fn func() error) error {
	err := fn()
	if err == nil {
		return nil
	}
	return fn()
}

// ---- 校验 / 清理 ----

// validateSlideContent 校验并就地修正视觉内容：填默认画布/配色、剔除非法元素。
func validateSlideContent(c *types.SlideContent) error {
	if c.Width <= 0 {
		c.Width = 1280
	}
	if c.Height <= 0 {
		c.Height = 720
	}
	if strings.TrimSpace(c.Background) == "" {
		c.Background = "#ffffff"
	}
	if strings.TrimSpace(c.Accent) == "" {
		c.Accent = "#14b8a6"
	}
	c.Elements = sanitizeSlideElements(c.Elements)
	if len(c.Elements) == 0 {
		return errors.New("slide content: no valid elements")
	}
	return nil
}

// sanitizeSlideElements 过滤非法/重复 id 的元素，保证 id 唯一。
func sanitizeSlideElements(in []types.SlideElement) []types.SlideElement {
	seen := make(map[string]bool, len(in))
	out := make([]types.SlideElement, 0, len(in))
	for _, el := range in {
		id := strings.TrimSpace(el.ID)
		if id == "" || seen[id] {
			continue
		}
		switch el.Type {
		case types.SlideElementText, types.SlideElementFormula, types.SlideElementShape, types.SlideElementList, types.SlideElementMermaid:
			// mermaid 元素复用 Content 存流程图源码，为空则无法渲染。
			if el.Type == types.SlideElementMermaid && strings.TrimSpace(el.Content) == "" {
				continue
			}
		case types.SlideElementImage:
			// 图片元素需有生成提示词；尺寸缺省时给默认值，保证前端可渲染、生成有宽高比。
			if strings.TrimSpace(el.Prompt) == "" && el.Src == "" {
				continue
			}
			if el.Width <= 0 {
				el.Width = 480
			}
			if el.Height <= 0 {
				el.Height = 360
			}
		case types.SlideElementChart:
			if !sanitizeChartElement(&el) {
				continue
			}
		case types.SlideElementFunctionPlot:
			if !sanitizeFunctionPlotElement(&el) {
				continue
			}
		default:
			continue
		}
		el.ID = id
		seen[id] = true
		out = append(out, el)
	}
	return out
}

// sanitizeSlideSteps 过滤空旁白步骤，剔除引用不存在元素或类型非法的动作。
// 白板类动作（laser / draw / clearBoard）按各自规则校验：
// laser 须有合法元素引用或坐标；draw 须有可渲染笔画。
func sanitizeSlideSteps(in []types.SlideStep, idSet map[string]bool, canvasW, canvasH int) []types.SlideStep {
	out := make([]types.SlideStep, 0, len(in))
	for _, st := range in {
		if strings.TrimSpace(st.Text) == "" {
			continue
		}
		acts := make([]types.SlideAction, 0, len(st.Actions))
		for _, a := range st.Actions {
			switch a.Type {
			case types.SlideActionUnderline, types.SlideActionHighlight, types.SlideActionBox:
				if !idSet[strings.TrimSpace(a.TargetElementID)] {
					continue
				}
				a.TargetElementID = strings.TrimSpace(a.TargetElementID)
				acts = append(acts, a)
			case types.SlideActionLaser:
				// 优先元素引用；两者皆合法时保留坐标供画布系兜底指向。
				switch {
				case idSet[strings.TrimSpace(a.TargetElementID)]:
					a.TargetElementID = strings.TrimSpace(a.TargetElementID)
					a.X = clampDrawCoord(a.X, canvasW)
					a.Y = clampDrawCoord(a.Y, canvasH)
				case a.X != 0 || a.Y != 0:
					a.TargetElementID = ""
					a.X = clampDrawCoord(a.X, canvasW)
					a.Y = clampDrawCoord(a.Y, canvasH)
				default:
					continue // 无元素引用也无坐标，无法定位
				}
				acts = append(acts, a)
			case types.SlideActionDraw:
				d := sanitizeSlideDrawing(a.Drawing, canvasW, canvasH)
				if d == nil {
					continue
				}
				a.Drawing = d
				a.TargetElementID = ""
				acts = append(acts, a)
			case types.SlideActionClearBoard:
				a.TargetElementID = ""
				a.Drawing = nil
				acts = append(acts, a)
			default:
				continue
			}
		}
		st.Actions = acts
		out = append(out, st)
	}
	return out
}

// clampDrawCoord 将坐标 clamp 到 [0, max] 区间。
func clampDrawCoord(v float64, max int) float64 {
	if v < 0 {
		return 0
	}
	if v > float64(max) {
		return float64(max)
	}
	return v
}

// sanitizeSlideDrawing 校验 draw 笔画并就地修正：kind/尺寸枚举、点集截断与坐标
// clamp、包围盒边界、text 必须有内容。非法返回 nil（由调用方剔除）。
func sanitizeSlideDrawing(d *types.SlideDrawing, canvasW, canvasH int) *types.SlideDrawing {
	if d == nil {
		return nil
	}
	switch d.Kind {
	case types.SlideDrawPen, types.SlideDrawLine, types.SlideDrawArrow:
		// 点集：line/arrow 恰好 2 点；pen 取前 64 点。
		if len(d.Points) == 0 {
			return nil
		}
		if d.Kind == types.SlideDrawPen {
			if len(d.Points) > 64 {
				d.Points = d.Points[:64]
			}
		} else if len(d.Points) < 2 {
			return nil
		}
		d.Points = clampSlidePoints(d.Points, canvasW, canvasH)
	case types.SlideDrawRect, types.SlideDrawCircle:
		if d.Width <= 0 || d.Height <= 0 {
			return nil
		}
		d.X = clampDrawCoord(d.X, canvasW)
		d.Y = clampDrawCoord(d.Y, canvasH)
	case types.SlideDrawText:
		if strings.TrimSpace(d.Content) == "" {
			return nil
		}
		if d.FontSize <= 0 || d.FontSize > 96 {
			d.FontSize = 24
		}
		d.X = clampDrawCoord(d.X, canvasW)
		d.Y = clampDrawCoord(d.Y, canvasH)
	default:
		return nil
	}
	switch d.Size {
	case types.SlideDrawSizeThin, types.SlideDrawSizeMedium, types.SlideDrawSizeThick:
	default:
		d.Size = types.SlideDrawSizeMedium // 粗细缺省
	}
	return d
}

// clampSlidePoints 将点集 clamp 到画布内。
func clampSlidePoints(pts [][2]float64, w, h int) [][2]float64 {
	for i := range pts {
		pts[i][0] = clampDrawCoord(pts[i][0], w)
		pts[i][1] = clampDrawCoord(pts[i][1], h)
	}
	return pts
}

// sanitizeChartElement 就地校验 chart 元素数据，非法返回 false（由调用方剔除）。
// 校验图表枚举、类目/系列非空、数据长度与类目对齐；pie 强制单系列。
func sanitizeChartElement(el *types.SlideElement) bool {
	switch el.Chart {
	case types.SlideChartBar, types.SlideChartLine, types.SlideChartPie:
	default:
		return false
	}
	if len(el.Categories) == 0 || len(el.Series) == 0 {
		return false
	}
	n := len(el.Categories)
	for i := range el.Categories {
		el.Categories[i] = strings.TrimSpace(el.Categories[i])
	}
	kept := make([]types.SlideChartSeries, 0, len(el.Series))
	for _, s := range el.Series {
		s.Name = strings.TrimSpace(s.Name)
		// 数据长度必须与类目对齐，否则语义错误，剔除该系列。
		if len(s.Values) != n {
			continue
		}
		kept = append(kept, s)
	}
	if len(kept) == 0 {
		return false
	}
	if el.Chart == types.SlideChartPie {
		// pie 只有一个系列；多余系列直接丢弃而非整体剔除。
		kept = kept[:1]
	}
	el.Series = kept
	if el.Width <= 0 {
		el.Width = 560
	}
	if el.Height <= 0 {
		el.Height = 360
	}
	return true
}

// functionExprIdentifiers 函数表达式允许的标识符（数学函数 + 常量），缺失/未知一律拒绝。
var functionExprIdentifiers = map[string]bool{
	"sin": true, "cos": true, "tan": true,
	"asin": true, "acos": true, "atan": true,
	"sinh": true, "cosh": true, "tanh": true,
	"abs": true, "sqrt": true, "cbrt": true,
	"log": true, "log2": true, "log10": true, "ln": true,
	"exp": true, "pow": true, "sign": true,
	"floor": true, "ceil": true, "round": true,
	"pi": true, "e": true, "x": true,
}

// validFunctionExpression 校验函数图像表达式为纯数学表达式：
// 字符白名单（数字 / x / 括号 / 运算符 / 小数点 / 逗号 / 空白）+ 标识符白名单。
// 非法表达式（含脚本注入、未知函数名等）直接剔除该曲线。表达式长度 <= 200。
func validFunctionExpression(expr string) bool {
	s := strings.TrimSpace(expr)
	if s == "" || len(s) > 200 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == 'x' || r == 'X':
		case r == '+' || r == '-' || r == '*' || r == '/' || r == '^':
		case r == '(' || r == ')':
		case r == '.' || r == ',':
		case r == ' ' || r == '\t':
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
		default:
			return false
		}
	}
	// 提取全部字母段，逐段核对白名单（大小写不敏感）
	start := -1
	for i := 0; i <= len(s); i++ {
		c := byte(' ')
		if i < len(s) {
			c = s[i]
		}
		isAlpha := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		switch {
		case isAlpha && start < 0:
			start = i
		case !isAlpha && start >= 0:
			tok := strings.ToLower(s[start:i])
			if !functionExprIdentifiers[tok] {
				return false
			}
			start = -1
		}
	}
	return true
}

// validFunctionRange 校验坐标窗口：有限数且下界 < 上界。
func validFunctionRange(r [2]float64) bool {
	if math.IsNaN(r[0]) || math.IsNaN(r[1]) || math.IsInf(r[0], 0) || math.IsInf(r[1], 0) {
		return false
	}
	return r[0] < r[1]
}

// sanitizeFunctionPlotElement 就地校验 functionPlot 元素数据，非法返回 false（由调用方剔除）。
// xRange 非法回退默认 [-6, 6]；yRange 非法归零（前端不设纵轴窗口，自适应）；
// 曲线逐条白名单校验、最多保留 4 条；无合法曲线则整体剔除。
func sanitizeFunctionPlotElement(el *types.SlideElement) bool {
	if !validFunctionRange(el.XRange) {
		el.XRange = [2]float64{-6, 6}
	}
	if !validFunctionRange(el.YRange) {
		el.YRange = [2]float64{}
	}
	kept := make([]types.SlideFunctionPlotCurve, 0, len(el.Curves))
	for _, c := range el.Curves {
		if len(kept) >= 4 {
			break
		}
		c.Expression = strings.TrimSpace(c.Expression)
		if !validFunctionExpression(c.Expression) {
			continue
		}
		c.Color = strings.TrimSpace(c.Color)
		// 颜色非法时清空，交由前端按 accent 派生色板分配。
		if c.Color != "" && !isValidHexColor(c.Color) {
			c.Color = ""
		}
		kept = append(kept, c)
	}
	if len(kept) == 0 {
		return false
	}
	el.Curves = kept
	el.Title = strings.TrimSpace(el.Title)
	if el.Width <= 0 {
		el.Width = 560
	}
	if el.Height <= 0 {
		el.Height = 360
	}
	return true
}

// isValidHexColor 校验合法 hex 颜色（#RRGGBB / #RRGGBBAA / #RGB）。
func isValidHexColor(c string) bool {
	if !strings.HasPrefix(c, "#") {
		return false
	}
	switch len(c) - 1 {
	case 3, 6, 8:
	default:
		return false
	}
	for _, r := range c[1:] {
		ok := r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
		if !ok {
			return false
		}
	}
	return true
}

// elementIDSet 收集元素 id 集合，供讲解动作引用校验。
func elementIDSet(c *types.SlideContent) map[string]bool {
	set := make(map[string]bool, len(c.Elements))
	for _, el := range c.Elements {
		set[el.ID] = true
	}
	return set
}

// ---- 上下文构造 ----

// slideSectionContext 生成课程结构上下文：按序列出各环节并标注已完成/当前环节，
// 供生成器自然承前启后（轻量连贯）。
func slideSectionContext(genCtx *types.GenerationContext, currentPos int) string {
	if len(genCtx.OutlineSections) == 0 {
		return ""
	}
	done := make(map[int]bool, len(genCtx.Done))
	for _, d := range genCtx.Done {
		done[d.Position] = true
	}
	var b strings.Builder
	b.WriteString("课程环节（按讲授顺序）：\n")
	for i, o := range genCtx.OutlineSections {
		title := strings.TrimSpace(o.Title)
		if title == "" {
			continue
		}
		pos := i + 1
		state := ""
		switch {
		case pos == currentPos:
			state = "【本环节，正在生成】"
		case done[pos]:
			state = "【已完成】"
		}
		kp := strings.TrimSpace(strings.Join(o.KnowledgePoints, "、"))
		if kp != "" {
			title += "（" + kp + "）"
		}
		fmt.Fprintf(&b, "%d. [%s] %s %s\n", pos, o.Type, title, state)
	}
	return strings.TrimRight(b.String(), "\n")
}

// slideElementSummary 生成供阶段二参考的元素清单摘要（id + 类型 + 内容要点）。
func slideElementSummary(c *types.SlideContent) string {
	var b strings.Builder
	for _, el := range c.Elements {
		b.WriteString(fmt.Sprintf("- %s [%s] @(%d,%d)", el.ID, el.Type, el.X, el.Y))
		switch el.Type {
		case types.SlideElementText:
			if el.Content != "" {
				b.WriteString(" 内容: ")
				b.WriteString(summarizeText(el.Content, 80))
			}
		case types.SlideElementFormula:
			if el.Content != "" {
				b.WriteString(" 公式: ")
				b.WriteString(summarizeText(el.Content, 60))
			}
		case types.SlideElementShape:
			b.WriteString(" shape=")
			b.WriteString(el.Shape)
			if el.Label != "" {
				b.WriteString(" 标注: ")
				b.WriteString(el.Label)
			}
		case types.SlideElementList:
			if len(el.Items) > 0 {
				b.WriteString(" 条目: ")
				b.WriteString(summarizeText(strings.Join(el.Items, " / "), 100))
			}
		case types.SlideElementMermaid:
			if el.Content != "" {
				b.WriteString(" 流程图源码: ")
				b.WriteString(summarizeText(el.Content, 120))
			}
		case types.SlideElementImage:
			if el.Prompt != "" {
				b.WriteString(" 图片: ")
				b.WriteString(summarizeText(el.Prompt, 100))
			}
		case types.SlideElementChart:
			b.WriteString(" chart=")
			b.WriteString(el.Chart)
			if el.Title != "" {
				b.WriteString(" 标题: ")
				b.WriteString(el.Title)
			}
			fmt.Fprintf(&b, " 类目: %s", strings.Join(el.Categories, " / "))
			for _, s := range el.Series {
				fmt.Fprintf(&b, " 系列[%s]: %v", s.Name, s.Values)
			}
		case types.SlideElementFunctionPlot:
			if el.Title != "" {
				b.WriteString(" 标题: ")
				b.WriteString(el.Title)
			}
			for _, c := range el.Curves {
				fmt.Fprintf(&b, " 曲线: y=%s", c.Expression)
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func summarizeText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ---- 取值辅助 ----

func courseTitle(genCtx *types.GenerationContext) string {
	if genCtx.Course != nil {
		return genCtx.Course.Title
	}
	return ""
}

func genCtxUserRequest(genCtx *types.GenerationContext) string {
	if genCtx.Course != nil {
		return genCtx.Course.Prompt
	}
	return ""
}

func sectionKnowledgePoints(section *types.Section) string {
	var kp []string
	_ = json.Unmarshal(section.KnowledgePoints, &kp)
	return strings.Join(kp, "、")
}

func sectionPrompt(section *types.Section) string {
	if section.Prompt != nil {
		return *section.Prompt
	}
	return ""
}
