package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// generate_outline.go 大纲生成域逻辑：
// 任务状态机（StartOutline/GetOutlineTask 内存任务表）+ LLM 生成核心（generateOutline，
// 提示词组装与解析）+ 大纲查询/历史版本管理（GetOutline/ListOutlineVersions/RevertOutline）。
// 课程 CRUD 见 course.go，CourseService 结构体与构造函数同在 course.go。

// 大纲生成参数
const (
	// maxDocRunes 进入提示词的参考文档总字符上限（超出按比例/先后截断）
	maxDocRunes = 24000
	// maxOutlineVersions 大纲历史版本保留上限（超出删除最旧）
	maxOutlineVersions = 10
)

// 大纲生成任务状态（内存态；重启丢失后由 DB 派生兜底）。
const (
	// taskStatusGenerating 大纲正在生成
	taskStatusGenerating = "generating"
	// taskStatusError 大纲生成失败
	taskStatusError = "error"
	// taskStatusIdle 未开始/丢失
	taskStatusIdle = "idle"
)

const generateOutlineTemperature = 0.2

// outlineTask 单课程大纲生成任务的内存状态。
type outlineTask struct {
	status  string
	message string
}

// ---- 大纲生成 ----

// StartOutline 启动大纲生成任务（后台异步执行）。feedback 可为空（等价重新生成）。
// 已在进行中重复触发返回 ErrOutlineGenerating；校验失败同步返回错误。
func (s *CourseService) StartOutline(ctx context.Context, userID, courseID types.ID, feedback string) error {
	course, err := s.courseRepo.GetByID(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return types.ErrCourseNotFound
		}
		return err
	}
	if strings.TrimSpace(course.Prompt) == "" {
		return types.ErrPromptRequired
	}
	if course.Status != types.CourseStatusDraft {
		return types.ErrOutlineAlreadyConfirmed
	}

	// 提取前置：在超时内幂等等待文档提取收敛（建课异步任务可能仍在进行）
	if _, _, err := s.docs.EnsureExtractedWithTimeout(ctx, courseID); err != nil {
		return err
	}

	s.outlineMu.Lock()
	if t := s.outlineTasks[courseID]; t != nil && t.status == taskStatusGenerating {
		s.outlineMu.Unlock()
		return types.ErrOutlineGenerating
	}
	s.outlineTasks[courseID] = &outlineTask{status: taskStatusGenerating}
	s.outlineMu.Unlock()

	fb := strings.TrimSpace(feedback)
	go s.runOutlineGeneration(course, fb)
	return nil
}

// runOutlineGeneration 后台执行大纲生成并落库；结果经 DB + 任务表可供轮询。
func (s *CourseService) runOutlineGeneration(course *types.Course, feedback string) {
	ctx := context.Background()
	if _, err := s.generateOutline(ctx, course.OwnerID, course.ID, feedback); err != nil {
		slog.Error("outline generation failed", "course_id", course.ID.String(), "error", err)
		s.outlineMu.Lock()
		t := s.outlineTasks[course.ID]
		if t != nil {
			t.status = taskStatusError
			t.message = types.ErrOutlineFailed.Msg
		}
		s.outlineMu.Unlock()
		return
	}
	s.outlineMu.Lock()
	delete(s.outlineTasks, course.ID)
	s.outlineMu.Unlock()
}

// GetOutlineTask 返回大纲生成任务状态；大纲已入库即视为 done（重启恢复安全）。
func (s *CourseService) GetOutlineTask(ctx context.Context, userID, courseID types.ID) (*types.OutlineTaskView, error) {
	if _, err := s.courseRepo.GetByID(ctx, userID, courseID); err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}

	o, oerr := s.outlineRepo.GetByCourse(ctx, courseID)
	if oerr == nil {
		var content types.OutlineContent
		if err := json.Unmarshal(o.Content, &content); err != nil {
			return nil, err
		}
		return &types.OutlineTaskView{
			Status:  "done",
			Outline: &types.OutlineView{Status: o.Status, Version: o.Version, Sections: content.Sections},
		}, nil
	}
	if !errors.Is(oerr, types.ErrNotFound) {
		return nil, oerr
	}

	s.outlineMu.Lock()
	t := s.outlineTasks[courseID]
	var tv types.OutlineTaskView
	switch {
	case t == nil:
		tv.Status = taskStatusIdle
	case t.status == taskStatusGenerating:
		tv.Status = taskStatusGenerating
	default:
		tv.Status = taskStatusError
		tv.Message = t.message
	}
	s.outlineMu.Unlock()
	return &tv, nil
}

// generateOutline 大纲生成核心：读取课程 + 参考文档 → LLM → 持久化（含历史版本快照）。
// feedback 非空且存在已生成大纲时，提示词附带已有大纲供参考调整。
func (s *CourseService) generateOutline(ctx context.Context, userID, courseID types.ID, feedback string) (*types.OutlineResult, error) {
	course, err := s.courseRepo.GetByID(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(course.Prompt) == "" {
		return nil, types.ErrPromptRequired
	}
	if course.Status != types.CourseStatusDraft {
		return nil, types.ErrOutlineAlreadyConfirmed
	}

	docsText, err := s.docs.LoadDocsText(ctx, courseID)
	if err != nil {
		return nil, err
	}

	// 读取当前大纲（可为空），供重新生成时参考已有大纲
	var existing *types.Outline
	if ex, gerr := s.outlineRepo.GetByCourse(ctx, courseID); gerr == nil {
		existing = ex
	} else if !errors.Is(gerr, types.ErrNotFound) {
		return nil, gerr
	}

	// 获取模型API
	var cfg model.ProviderConfig
	if course.ModelConfigID != nil && *course.ModelConfigID != types.NilID {
		cfg, err = s.modelCfgSvc.ResolveByID(ctx, userID, *course.ModelConfigID)
	} else {
		cfg, err = s.modelCfgSvc.ResolveDefault(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	if cfg.Model == "" || cfg.APIKey == "" {
		return nil, types.ErrNoModelConfig
	}
	client := s.registry.NewLLM(cfg)
	if client == nil {
		return nil, types.ErrNoModelConfig
	}

	// 构建提示词
	messages, err := buildOutlineMessages(course.Prompt, docsText, normalizeOutlineCount(course.OutlineCount),
		existingOutlineText(existing), feedback)
	if err != nil {
		return nil, err
	}
	temp := generateOutlineTemperature
	slog.Debug("generating outline for: ", "course", course.ID, "prompt", course.Prompt)
	// 模型生成大纲（失败按策略重试）
	var resp *model.ChatResponse
	if cerr := util.Retry(ctx, s.retry, func() error {
		r, gerr := client.Chat(ctx, model.ChatRequest{
			Messages:    messages,
			Temperature: &temp,
			Thinking:    course.Thinking,
		})
		resp = r
		return gerr
	}); cerr != nil {
		return nil, fmt.Errorf("generate outline: %w", cerr)
	}

	// 解析模型生成内容
	var raw types.OutlineLLMResult
	if err := util.ExtractJSON(resp.Content, &raw); err != nil {
		return nil, types.ErrOutlineFailed
	}
	sections := normalizeSections(raw.Sections)
	if len(sections) == 0 {
		return nil, types.ErrOutlineFailed
	}

	content, err := json.Marshal(types.OutlineContent{Sections: sections})
	if err != nil {
		return nil, err
	}

	// 版本号：当前始终为历史最大版本，每次写入都推进 +1（含回退场景）。
	version := 1
	if existing != nil {
		version = existing.Version + 1
	}

	// 持久化大纲 + 写入历史快照 + 回填标题（同一事务保证一致）
	err = s.tm.Transaction(ctx, func(ctx context.Context) error {
		by := userID
		outlineID := types.NilID
		if existing != nil {
			outlineID = existing.ID
			if uerr := s.outlineRepo.UpdateContentVersion(ctx, courseID, datatypes.JSON(content), version, &by); uerr != nil {
				return uerr
			}
		} else {
			o := &types.Outline{
				CourseID: courseID,
				Content:  datatypes.JSON(content),
				Status:   types.OutlineStatusDraft,
				Version:  version,
				CreateBy: &by,
				UpdateBy: &by,
			}
			if cerr := s.outlineRepo.Create(ctx, o); cerr != nil {
				return cerr
			}
			outlineID = o.ID
		}
		if herr := s.historyRepo.Create(ctx, &types.OutlineHistory{
			OutlineID: outlineID,
			Version:   version,
			Title:     strings.TrimSpace(raw.Title),
			Content:   datatypes.JSON(content),
			Feedback:  feedback,
		}); herr != nil {
			return herr
		}
		if perr := s.historyRepo.Prune(ctx, outlineID, maxOutlineVersions); perr != nil {
			return perr
		}
		if strings.TrimSpace(raw.Title) != "" {
			return s.courseRepo.UpdateTitle(ctx, courseID, strings.TrimSpace(raw.Title))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.OutlineResult{Title: raw.Title, Sections: sections}, nil
}

// GetOutline 返回已保存大纲；生成失败/未生成返回 ErrOutlineNotFound。
func (s *CourseService) GetOutline(ctx context.Context, userID, courseID types.ID) (*types.OutlineView, error) {
	if _, err := s.courseRepo.GetByID(ctx, userID, courseID); err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}
	o, err := s.outlineRepo.GetByCourse(ctx, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrOutlineNotFound
		}
		return nil, err
	}
	var content types.OutlineContent
	if err := json.Unmarshal(o.Content, &content); err != nil {
		return nil, err
	}
	return &types.OutlineView{Status: o.Status, Version: o.Version, Sections: content.Sections}, nil
}

// ListOutlineVersions 返回大纲历史版本列表（最新在前，标注当前版本）。
func (s *CourseService) ListOutlineVersions(ctx context.Context, userID, courseID types.ID) ([]types.OutlineVersionView, error) {
	o, err := s.getDraftOutline(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	rows, err := s.historyRepo.ListByOutline(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	items := make([]types.OutlineVersionView, 0, len(rows))
	for _, h := range rows {
		items = append(items, types.OutlineVersionView{
			Version:   h.Version,
			Title:     h.Title,
			Feedback:  h.Feedback,
			Current:   h.Version == o.Version,
			CreatedAt: h.CreateAt,
		})
	}
	return items, nil
}

// RevertOutline 回退大纲到指定历史版本（回退也作为新版本快照写入，保持历史可追溯）。
func (s *CourseService) RevertOutline(ctx context.Context, userID, courseID types.ID, version int) (*types.OutlineView, error) {
	o, err := s.getDraftOutline(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	target, err := s.historyRepo.GetByVersion(ctx, o.ID, version)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrOutlineVersionNotFound
		}
		return nil, err
	}

	newVersion := o.Version + 1
	err = s.tm.Transaction(ctx, func(ctx context.Context) error {
		by := userID
		if uerr := s.outlineRepo.UpdateContentVersion(ctx, courseID, target.Content, newVersion, &by); uerr != nil {
			return uerr
		}
		if herr := s.historyRepo.Create(ctx, &types.OutlineHistory{
			OutlineID: o.ID,
			Version:   newVersion,
			Title:     target.Title,
			Content:   target.Content,
			Feedback:  fmt.Sprintf("回退到第 %d 版", version),
		}); herr != nil {
			return herr
		}
		if perr := s.historyRepo.Prune(ctx, o.ID, maxOutlineVersions); perr != nil {
			return perr
		}
		if strings.TrimSpace(target.Title) != "" {
			return s.courseRepo.UpdateTitle(ctx, courseID, strings.TrimSpace(target.Title))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetOutline(ctx, userID, courseID)
}

// getDraftOutline 校验课程归属 + draft 状态，返回其大纲。
func (s *CourseService) getDraftOutline(ctx context.Context, userID, courseID types.ID) (*types.Outline, error) {
	course, err := s.courseRepo.GetByID(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}
	if course.Status != types.CourseStatusDraft {
		return nil, types.ErrOutlineAlreadyConfirmed
	}
	o, err := s.outlineRepo.GetByCourse(ctx, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrOutlineNotFound
		}
		return nil, err
	}
	return o, nil
}

// existingOutlineText 将当前大纲格式化为 LLM 可读的 Markdown 列表。
func existingOutlineText(o *types.Outline) string {
	if o == nil {
		return ""
	}
	var content types.OutlineContent
	if err := json.Unmarshal(o.Content, &content); err != nil || len(content.Sections) == 0 {
		return ""
	}
	var b strings.Builder
	for i, s := range content.Sections {
		fmt.Fprintf(&b, "%d. [%s] %s", i+1, s.Type, s.Title)
		if len(s.KnowledgePoints) > 0 {
			b.WriteString(" — 知识点：")
			b.WriteString(strings.Join(s.KnowledgePoints, "；"))
		}
		b.WriteString("\n")
		if strings.TrimSpace(s.Description) != "" {
			b.WriteString("   描述：")
			b.WriteString(strings.TrimSpace(s.Description))
			b.WriteString("\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// outlineUserData 用户提示词模板的填充字段。
type outlineUserData struct {
	// Requirement 用户课程要求
	Requirement string
	// DocsSummary 参考文件摘要（当前以文档完整内容填充）
	DocsSummary string
}

// outlineRegenerateUserData 重新生成提示词模板的填充字段。
type outlineRegenerateUserData struct {
	// Requirement 用户课程要求
	Requirement string
	// DocsSummary 参考文件摘要
	DocsSummary string
	// ExistingOutline 已生成的大纲（Markdown 列表；可为空）
	ExistingOutline string
	// Feedback 修改意见
	Feedback string
}

// outlineSystemData 系统提示词模板的填充字段。
type outlineSystemData struct {
	// SectionCount 大纲环节数量上限
	SectionCount int
}

// buildOutlineMessages 用 outline.md 渲染 system 提示词（注入环节数量上限）。
// user 消息按场景选择模板：有修改意见用 regenerate 模板（附带已有大纲 + 意见），
// 否则用首次生成模板。
func buildOutlineMessages(prompt, docsText string, sectionCount int, existingText, feedback string) ([]model.ChatMessage, error) {
	var sys bytes.Buffer
	if err := outlineSystemTpl.Execute(&sys, outlineSystemData{SectionCount: sectionCount}); err != nil {
		return nil, fmt.Errorf("render outline system prompt: %w", err)
	}

	var user bytes.Buffer
	var uerr error
	if strings.TrimSpace(feedback) == "" {
		uerr = outlineUserTpl.Execute(&user, outlineUserData{
			Requirement: prompt,
			DocsSummary: docsText,
		})
	} else {
		uerr = outlineRegenerateUserTpl.Execute(&user, outlineRegenerateUserData{
			Requirement:     prompt,
			DocsSummary:     docsText,
			ExistingOutline: existingText,
			Feedback:        feedback,
		})
	}
	if uerr != nil {
		return nil, fmt.Errorf("render outline user prompt: %w", uerr)
	}
	return []model.ChatMessage{
		{Role: model.RoleSystem, Content: sys.String()},
		{Role: model.RoleUser, Content: user.String()},
	}, nil
}

// normalizeSections 校验并过滤非法环节（类型须合法、标题非空）。
func normalizeSections(in []types.OutlineSection) []types.OutlineSection {
	out := make([]types.OutlineSection, 0, len(in))
	for _, s := range in {
		title := strings.TrimSpace(s.Title)
		if title == "" {
			continue
		}
		switch s.Type {
		case types.SectionTypeSlide, types.SectionTypeQuiz,
			types.SectionTypeDemo3D, types.SectionTypeDemoFunction, types.SectionTypeDemoBasic:
		default:
			continue
		}
		s.Title = title
		s.Description = strings.TrimSpace(s.Description)
		s.KnowledgePoints = trimAll(s.KnowledgePoints)
		out = append(out, s)
	}
	return out
}

func trimAll(ss []string) []string {
	if len(ss) == 0 {
		return ss
	}
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// normalizeOutlineCount 归一化大纲环节数量上限：小于下限（含未设 0）回退默认，超过上限截断。
func normalizeOutlineCount(n int) int {
	if n < types.MinOutlineCount {
		return types.DefaultOutlineCount
	}
	if n > types.MaxOutlineCount {
		return types.MaxOutlineCount
	}
	return n
}
