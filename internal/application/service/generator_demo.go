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

// Demo Basic 环节单阶段生成：一次 LLM 调用产出三段内容（style/body/script，大模型只出逻辑），
// 校验清洗后用 go:embed 的 HTML 骨架拼接成完整可运行页面（含 CSP 禁网络），写入 section.content。
// 解析/校验失败自动重试一次（共 2 次尝试）；不设 max_tokens，交模型默认上限。

// demoBasicUserData Demo Basic user 模板填充字段。
type demoBasicUserData struct {
	CourseTitle      string
	SectionTitle     string
	KnowledgePoints  string
	Prompt           string
	PreviousSections string
	DocsSummary      string
	UserRequest      string
}

// demoBasicGenerator Demo Basic 环节内容生成器。模型客户端与上下文由 SectionService
// 通过 GenerationContext 注入；无额外持久依赖。
type demoBasicGenerator struct{}

// demoBasicLLMResult LLM 产出的三段内容（拼接前，无骨架/标签包裹）。
type demoBasicLLMResult struct {
	Style  string `json:"style"`
	Body   string `json:"body"`
	Script string `json:"script"`
}

// demoBasicContent 落库的 content 结构（与前端 DemoContent {code} 对齐）。
type demoBasicContent struct {
	Code string `json:"code"`
}

// Generate 单次调用 LLM 产出三段逻辑，拼接完整 HTML 后写入 section.Content。
func (g *demoBasicGenerator) Generate(ctx context.Context, section *types.Section, genCtx *types.GenerationContext) error {
	if genCtx == nil || genCtx.Client == nil {
		return errors.New("demo basic generator: missing llm client")
	}

	msgData := demoBasicUserData{
		CourseTitle:      courseTitle(genCtx),
		SectionTitle:     section.Title,
		KnowledgePoints:  sectionKnowledgePoints(section),
		Prompt:           sectionPrompt(section),
		PreviousSections: slideSectionContext(genCtx, section.Position),
		DocsSummary:      strings.TrimSpace(genCtx.DocsText),
		UserRequest:      genCtxUserRequest(genCtx),
	}
	messages, err := renderDemoBasicMessages(demoBasicUserTpl, msgData, demoBasicSystemPrompt)
	if err != nil {
		return err
	}

	var result *demoBasicLLMResult
	err = retryCall(func() error {
		slog.Debug("generating demo basic code")
		resp, cerr := chatOnce(ctx, genCtx.Client, messages, 0.3, genCtx.Thinking)
		if cerr != nil {
			return cerr
		}
		var out demoBasicLLMResult
		if xerr := util.ExtractJSON(resp.Content, &out); xerr != nil {
			slog.Warn("invalid demo basic content", "content", resp.Content)
			return fmt.Errorf("parse demo basic json: %w", xerr)
		}
		if !validDemoBasic(out) {
			return errors.New("demo basic: empty style/body/script after sanitize")
		}
		result = &out
		slog.Debug("generate demo basic code done")
		return nil
	})
	if err != nil {
		slog.Warn("demo basic generation failed", "section_id", section.ID.String(), "error", err)
		return err
	}

	content, err := json.Marshal(demoBasicContent{Code: assembleDemoBasic(result)})
	if err != nil {
		return fmt.Errorf("marshal demo basic content: %w", err)
	}
	section.Content = datatypes.JSON(content)
	return nil
}

// renderDemoBasicMessages 渲染 Demo Basic user 模板并拼接 system 提示词。
func renderDemoBasicMessages(tpl *template.Template, data any, system string) ([]model.ChatMessage, error) {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render demo basic prompt: %w", err)
	}
	return []model.ChatMessage{
		{Role: model.RoleSystem, Content: system},
		{Role: model.RoleUser, Content: buf.String()},
	}, nil
}

// validDemoBasic 三段内容至少有一段非空，否则视为无效。
func validDemoBasic(r demoBasicLLMResult) bool {
	return strings.TrimSpace(r.Style) != "" ||
		strings.TrimSpace(r.Body) != "" ||
		strings.TrimSpace(r.Script) != ""
}

// assembleDemoBasic 用 HTML 骨架拼接三段内容为完整页面。
// 直接以字符串替换占位符（避免 text/template 与代码中的 {{ }} 冲突）。
func assembleDemoBasic(r *demoBasicLLMResult) string {
	html := demoBasicTemplate
	html = strings.Replace(html, "__STYLE__", r.Style, 1)
	html = strings.Replace(html, "__BODY__", r.Body, 1)
	html = strings.Replace(html, "__SCRIPT__", r.Script, 1)
	return html
}
