package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
		out = sanitizeSlideSteps(out, idSet)
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
func sanitizeSlideSteps(in []types.SlideStep, idSet map[string]bool) []types.SlideStep {
	out := make([]types.SlideStep, 0, len(in))
	for _, st := range in {
		if strings.TrimSpace(st.Text) == "" {
			continue
		}
		acts := make([]types.SlideAction, 0, len(st.Actions))
		for _, a := range st.Actions {
			switch a.Type {
			case types.SlideActionUnderline, types.SlideActionHighlight, types.SlideActionBox:
			default:
				continue
			}
			if !idSet[strings.TrimSpace(a.TargetElementID)] {
				continue
			}
			a.TargetElementID = strings.TrimSpace(a.TargetElementID)
			acts = append(acts, a)
		}
		st.Actions = acts
		out = append(out, st)
	}
	return out
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
