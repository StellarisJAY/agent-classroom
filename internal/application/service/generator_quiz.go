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

// Quiz 环节单阶段生成：一次 LLM 调用产出题目数组（type/stem/options/answers/explanations），
// 校验清洗后整表替换写入 question 表（见 docs/数据库设计.md 3.6）。
// 解析/校验失败自动重试一次（共 2 次尝试）；不设 max_tokens，交模型默认上限。

// quizUserData Quiz user 模板填充字段。
type quizUserData struct {
	CourseTitle      string
	SectionTitle     string
	KnowledgePoints  string
	Prompt           string
	PreviousSections string
	DocsSummary      string
	UserRequest      string
}

// quizGenerator Quiz 环节内容生成器。题目持久化依赖 questionRepo；无视觉 content。
type quizGenerator struct {
	questionRepo types.QuestionRepo
}

// quizLLMQuestion LLM 产出的中间题目结构（落库前无 id/position）。
type quizLLMQuestion struct {
	Type         string   `json:"type"`
	Stem         string   `json:"stem"`
	Options      []string `json:"options"`
	Answers      []int    `json:"answers"`
	Explanations []string `json:"explanations"`
}

// 题目数量控制：LLM 引导 3~6 题；清洗后至少保留 minQuizQuestions 才判成功。
const (
	maxQuizQuestions = 6
	minQuizQuestions = 1
)

// Generate 单阶段生成题目并整表替换写入 question 表。
func (g *quizGenerator) Generate(ctx context.Context, section *types.Section, genCtx *types.GenerationContext) error {
	if g == nil || g.questionRepo == nil {
		return errors.New("quiz generator: missing question repo")
	}
	if genCtx == nil || genCtx.Client == nil {
		return errors.New("quiz generator: missing llm client")
	}

	msgData := quizUserData{
		CourseTitle:      courseTitle(genCtx),
		SectionTitle:     section.Title,
		KnowledgePoints:  sectionKnowledgePoints(section),
		Prompt:           sectionPrompt(section),
		PreviousSections: slideSectionContext(genCtx, section.Position),
		DocsSummary:      strings.TrimSpace(genCtx.DocsText),
		UserRequest:      genCtxUserRequest(genCtx),
	}
	messages, err := renderQuizMessages(quizUserTpl, msgData, quizSystemPrompt)
	if err != nil {
		return err
	}

	var questions []quizLLMQuestion
	err = retryCall(func() error {
		slog.Debug("generating quiz questions")
		resp, cerr := chatOnce(ctx, genCtx.Client, messages, 0.3, genCtx.Thinking)
		if cerr != nil {
			return cerr
		}
		var out []quizLLMQuestion
		if xerr := util.ExtractJSON(resp.Content, &out); xerr != nil {
			slog.Warn("invalid quiz content", "content", resp.Content)
			return fmt.Errorf("parse quiz json: %w", xerr)
		}
		questions = sanitizeQuizQuestions(out)
		if len(questions) < minQuizQuestions {
			return errors.New("quiz: no valid questions after sanitize")
		}
		slog.Debug("generate quiz questions done", "count", len(questions))
		return nil
	})
	if err != nil {
		slog.Warn("quiz generation failed", "section_id", section.ID.String(), "error", err)
		return err
	}

	rows, err := buildQuestionRows(questions, genCtx)
	if err != nil {
		return err
	}
	return g.questionRepo.ReplaceBySection(ctx, section.ID, rows)
}

// renderQuizMessages 渲染 Quiz user 模板并拼接 system 提示词。
func renderQuizMessages(tpl *template.Template, data any, system string) ([]model.ChatMessage, error) {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render quiz prompt: %w", err)
	}
	return []model.ChatMessage{
		{Role: model.RoleSystem, Content: system},
		{Role: model.RoleUser, Content: buf.String()},
	}, nil
}

// ---- 校验 / 清理 ----

// sanitizeQuizQuestions 清洗并截断题目：剔除非法/越界的题，保留至多 maxQuizQuestions 道。
func sanitizeQuizQuestions(in []quizLLMQuestion) []quizLLMQuestion {
	out := make([]quizLLMQuestion, 0, len(in))
	for _, q := range in {
		if !validQuizQuestion(q) {
			continue
		}
		out = append(out, q)
		if len(out) >= maxQuizQuestions {
			break
		}
	}
	return out
}

// validQuizQuestion 校验单题结构是否符合出题规则。
func validQuizQuestion(q quizLLMQuestion) bool {
	stem := strings.TrimSpace(q.Stem)
	if stem == "" {
		return false
	}
	nOpts := len(q.Options)
	if nOpts < 2 {
		return false
	}
	if len(q.Explanations) != nOpts {
		return false
	}
	if q.Type == types.QuestionTypeSingle {
		if nOpts > 6 || len(q.Answers) != 1 {
			return false
		}
	} else if q.Type == types.QuestionTypeMultiple {
		if nOpts < 3 || nOpts > 6 || len(q.Answers) < 2 {
			return false
		}
	} else {
		return false
	}
	seen := make(map[int]bool, len(q.Answers))
	for _, a := range q.Answers {
		if a < 0 || a >= nOpts || seen[a] {
			return false
		}
		seen[a] = true
	}
	for _, e := range q.Explanations {
		if strings.TrimSpace(e) == "" {
			return false
		}
	}
	return true
}

// ---- 落库 ----

// buildQuestionRows 将清洗后的题目转换为 Question 实体（含 jsonb 数组与 position）。
func buildQuestionRows(questions []quizLLMQuestion, genCtx *types.GenerationContext) ([]types.Question, error) {
	rows := make([]types.Question, 0, len(questions))
	for i, q := range questions {
		opts, err := json.Marshal(q.Options)
		if err != nil {
			return nil, err
		}
		ans, err := json.Marshal(q.Answers)
		if err != nil {
			return nil, err
		}
		exps, err := json.Marshal(q.Explanations)
		if err != nil {
			return nil, err
		}
		var by *types.ID
		if genCtx != nil && genCtx.Course != nil {
			id := genCtx.Course.OwnerID
			by = &id
		}
		rows = append(rows, types.Question{
			Type:         strings.TrimSpace(q.Type),
			Position:     i + 1,
			Stem:         strings.TrimSpace(q.Stem),
			Options:      datatypes.JSON(opts),
			Answers:      datatypes.JSON(ans),
			Explanations: datatypes.JSON(exps),
			CreateBy:     by,
			UpdateBy:     by,
		})
	}
	return rows, nil
}
