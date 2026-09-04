package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
	"github.com/StellarisJAY/agent-classroom/internal/util"
)

// 大纲生成参数
const (
	// maxDocRunes 进入提示词的参考文档总字符上限（超出按比例/先后截断）
	maxDocRunes = 24000
)

// 默认分页参数与上限
const (
	defaultCoursePageSize = 20
	maxCoursePageSize     = 100
)

// CourseService 课程业务实现。
type CourseService struct {
	courseRepo  types.CourseRepo
	outlineRepo types.OutlineRepo
	docRepo     types.DocumentRepo
	tm          types.TransactionManager
	storage     types.Storage
	modelCfgSvc types.ModelConfigService
	registry    *model.Registry
}

var _ types.CourseService = (*CourseService)(nil)

// NewCourseService 创建课程业务实现。
func NewCourseService(
	courseRepo types.CourseRepo,
	outlineRepo types.OutlineRepo,
	docRepo types.DocumentRepo,
	tm types.TransactionManager,
	storage types.Storage,
	modelCfgSvc types.ModelConfigService,
	registry *model.Registry,
) types.CourseService {
	return &CourseService{
		courseRepo:  courseRepo,
		outlineRepo: outlineRepo,
		docRepo:     docRepo,
		tm:          tm,
		storage:     storage,
		modelCfgSvc: modelCfgSvc,
		registry:    registry,
	}
}

// ---- 列表 ----

func (s *CourseService) List(ctx context.Context, userID types.ID, req *types.CourseListReq) (*types.CourseListResp, error) {
	scope := types.CourseScopeAll
	if req != nil && req.Scope != "" {
		scope = req.Scope
	}
	if scope != types.CourseScopeMine && scope != types.CourseScopePublic {
		scope = types.CourseScopeAll
	}

	page, size := 1, defaultCoursePageSize
	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.PageSize > 0 {
			size = req.PageSize
		}
	}
	if size > maxCoursePageSize {
		size = maxCoursePageSize
	}

	progress := ""
	if req != nil {
		progress = strings.TrimSpace(req.Progress)
	}

	rows, total, err := s.courseRepo.List(ctx, userID, types.CourseListFilter{
		Scope:    scope,
		Keyword:  strings.TrimSpace(keywordOf(req)),
		Progress: progress,
		Page:     page,
		PageSize: size,
		Offset:   (page - 1) * size,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.CourseListItem, 0, len(rows))
	for i := range rows {
		c := &rows[i].Course
		items = append(items, types.CourseListItem{
			ID:        c.ID,
			Title:     c.Title,
			Status:    c.Status,
			IsPublic:  c.IsPublic,
			Owned:     c.OwnerID == userID,
			Progress:  rows[i].ProgressStatus,
			CreatedAt: c.CreateAt,
			UpdatedAt: c.UpdateAt,
		})
	}

	return &types.CourseListResp{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: size,
	}, nil
}

// ---- 创建 ----

// Create 创建草稿课程并保存参考文档。提取文本不入库；文件经 storage 落盘并记录元数据。
func (s *CourseService) Create(ctx context.Context, userID types.ID, req *types.CreateCourseReq) (*types.CourseCreateResp, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, types.ErrPromptRequired
	}
	// 前置校验全部文件格式与大小，避免写入半途失败
	for _, f := range req.Files {
		if _, err := util.ExtractText(f.Name, f.Data); err != nil {
			return nil, err
		}
	}

	course := &types.Course{
		OwnerID:       userID,
		Title:         "",
		Prompt:        prompt,
		Status:        types.CourseStatusDraft,
		ModelConfigID: req.ModelConfigID,
		Thinking:      normalizeThinking(req.Thinking),
		CreateBy:      &userID,
	}

	err := s.tm.Transaction(ctx, func(ctx context.Context) error {
		if err := s.courseRepo.Create(ctx, course); err != nil {
			return err
		}
		for _, f := range req.Files {
			key := course.ID.String() + "/" + filepath.Base(f.Name)
			url, err := s.storage.Put(ctx, key, strings.NewReader(string(f.Data)))
			if err != nil {
				return err
			}
			by := userID
			if err := s.docRepo.Create(ctx, &types.Document{
				CourseID: course.ID,
				Filename: f.Name,
				URL:      url,
				CreateBy: &by,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.CourseCreateResp{
		ID:     course.ID,
		Title:  course.Title,
		Prompt: course.Prompt,
		Status: course.Status,
	}, nil
}

// ---- 大纲生成 ----

func (s *CourseService) GenerateOutline(ctx context.Context, userID, courseID types.ID) (*types.OutlineResult, error) {
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

	docsText, err := loadDocumentsText(ctx, s.docRepo, s.storage, courseID)
	if err != nil {
		return nil, err
	}

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

	messages, err := buildOutlineMessages(course.Prompt, docsText)
	if err != nil {
		return nil, err
	}
	temp := 0.3
	resp, err := client.Chat(ctx, model.ChatRequest{
		Messages:    messages,
		Temperature: &temp,
		Thinking:    course.Thinking,
	})
	if err != nil {
		return nil, fmt.Errorf("generate outline: %w", err)
	}

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

	// 持久化大纲 + 回填标题（同一事务保证一致）
	err = s.tm.Transaction(ctx, func(ctx context.Context) error {
		by := userID
		if err := s.persistOutline(ctx, courseID, datatypes.JSON(content), &by); err != nil {
			return err
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
	return &types.OutlineView{Status: o.Status, Sections: content.Sections}, nil
}

// persistOutline 存在则覆盖，不存在则新建。
func (s *CourseService) persistOutline(ctx context.Context, courseID types.ID, content datatypes.JSON, by *types.ID) error {
	if _, err := s.outlineRepo.GetByCourse(ctx, courseID); err == nil {
		return s.outlineRepo.UpdateContentStatus(ctx, courseID, content, types.OutlineStatusDraft, by)
	} else if !errors.Is(err, types.ErrNotFound) {
		return err
	}
	return s.outlineRepo.Create(ctx, &types.Outline{
		CourseID: courseID,
		Content:  content,
		Status:   types.OutlineStatusDraft,
		CreateBy: by,
		UpdateBy: by,
	})
}

// outlineUserData 用户提示词模板的填充字段。
type outlineUserData struct {
	// Requirement 用户课程要求
	Requirement string
	// DocsSummary 参考文件摘要（当前以文档完整内容填充）
	DocsSummary string
}

// buildOutlineMessages 用 outline_user.md 模板渲染用户消息（填充课程要求与文档摘要），
// 并拼接已外置的 system 提示词。
func buildOutlineMessages(prompt, docsText string) ([]model.ChatMessage, error) {
	var buf bytes.Buffer
	if err := outlineUserTpl.Execute(&buf, outlineUserData{
		Requirement: prompt,
		DocsSummary: docsText,
	}); err != nil {
		return nil, fmt.Errorf("render outline user prompt: %w", err)
	}
	return []model.ChatMessage{
		{Role: model.RoleSystem, Content: outlineSystemPrompt},
		{Role: model.RoleUser, Content: buf.String()},
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
		case types.SectionTypeSlide, types.SectionTypeQuiz, types.SectionTypeDemo:
		default:
			continue
		}
		s.Title = title
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

func keywordOf(req *types.CourseListReq) string {
	if req == nil {
		return ""
	}
	return req.Keyword
}

// normalizeThinking 归一化思考限制取值；空或非法一律回退为 default。
func normalizeThinking(v string) string {
	switch v {
	case model.ThinkingOff, model.ThinkingDefault, model.ThinkingMax:
		return v
	default:
		return model.ThinkingDefault
	}
}
