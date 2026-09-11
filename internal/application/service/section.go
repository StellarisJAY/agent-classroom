package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// ---- 内容生成器占位实现（demo 三种类型均未落地，仅置占位产物并标记完成） ----

// stubGenerator demo 环节（demo_3d / demo_function / demo_basic）的内容生成器占位：
// 不调用 LLM，仅写入占位产物，用于打通「确认 → 串行生成 → 完成」全链路。
// 骨架完成后按类型分别替换为真实生成器实现。
type stubGenerator struct {
	sectionType string
}

func (g *stubGenerator) Generate(_ context.Context, section *types.Section, _ *types.GenerationContext) error {
	slog.Info("section generator stub (not implemented)",
		"type", g.sectionType, "section_id", section.ID.String(), "title", section.Title)
	content, err := json.Marshal(map[string]any{"type": g.sectionType, "stub": true, "generated": false})
	if err != nil {
		return err
	}
	section.Content = datatypes.JSON(content)
	return nil
}

// ---- 进程内生成运行权（简化实现：单进程内单课程只跑一个串行循环） ----

// sectionRuns 单课程内容生成循环的运行状态；无分布式 worker，仅防同一课程重复起循环。
type sectionRuns struct {
	mu      sync.Mutex
	running map[types.ID]bool
}

func newSectionRuns() *sectionRuns {
	return &sectionRuns{running: make(map[types.ID]bool)}
}

// tryStart 抢占某课程的运行权；已在运行返回 false。
func (r *sectionRuns) tryStart(courseID types.ID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running[courseID] {
		return false
	}
	r.running[courseID] = true
	return true
}

// finish 释放运行权。
func (r *sectionRuns) finish(courseID types.ID) {
	r.mu.Lock()
	delete(r.running, courseID)
	r.mu.Unlock()
}

// ---- SectionService ----

// SectionService 课程环节内容生成业务实现。
type SectionService struct {
	courseRepo   types.CourseRepo
	outlineRepo  types.OutlineRepo
	sectionRepo  types.SectionRepo
	questionRepo types.QuestionRepo
	tm           types.TransactionManager
	docRepo      types.DocumentRepo
	storage      types.Storage
	modelCfgSvc  types.ModelConfigService
	registry     *model.Registry
	generators   map[string]types.SectionContentGenerator
	runs         *sectionRuns
}

var _ types.SectionService = (*SectionService)(nil)

// NewSectionService 创建环节内容生成业务实现。
func NewSectionService(
	courseRepo types.CourseRepo,
	outlineRepo types.OutlineRepo,
	sectionRepo types.SectionRepo,
	questionRepo types.QuestionRepo,
	tm types.TransactionManager,
	docRepo types.DocumentRepo,
	storage types.Storage,
	modelCfgSvc types.ModelConfigService,
	registry *model.Registry,
) types.SectionService {
	return &SectionService{
		courseRepo:   courseRepo,
		outlineRepo:  outlineRepo,
		sectionRepo:  sectionRepo,
		questionRepo: questionRepo,
		tm:           tm,
		docRepo:      docRepo,
		storage:      storage,
		modelCfgSvc:  modelCfgSvc,
		registry:     registry,
		generators: map[string]types.SectionContentGenerator{
			types.SectionTypeSlide: &slideGenerator{},
			types.SectionTypeQuiz:  &quizGenerator{questionRepo: questionRepo},
			// demo 三种类型拆分为独立环节类型，各自维护生成流程、提示词与代码模板。
			// demo_basic 已实现真实生成（后端拼接模板 + LLM 只出逻辑）；
			// demo_3d / demo_function 仍为占位，后续按同样模式补全。
			types.SectionTypeDemo3D:       &stubGenerator{sectionType: types.SectionTypeDemo3D},
			types.SectionTypeDemoFunction: &stubGenerator{sectionType: types.SectionTypeDemoFunction},
			types.SectionTypeDemoBasic:    &demoBasicGenerator{},
		},
		runs: newSectionRuns(),
	}
}

// ---- 确认大纲 ----

func (s *SectionService) ConfirmOutline(ctx context.Context, userID, courseID types.ID, req *types.ConfirmOutlineReq) ([]types.SectionProgress, error) {
	if req == nil || len(req.Sections) == 0 {
		return nil, types.ErrInvalidRequest
	}
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
	if _, err := s.outlineRepo.GetByCourse(ctx, courseID); err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrOutlineNotFound
		}
		return nil, err
	}

	sections := normalizeSections(req.Sections)
	if len(sections) == 0 {
		return nil, types.ErrInvalidRequest
	}
	content, err := json.Marshal(types.OutlineContent{Sections: sections})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	by := userID
	rows := make([]types.Section, 0, len(sections))
	for i, os := range sections {
		kp, _ := json.Marshal(os.KnowledgePoints)
		row := types.Section{
			CourseID:        courseID,
			Position:        i + 1,
			Type:            os.Type,
			Title:           os.Title,
			KnowledgePoints: datatypes.JSON(kp),
			Status:          types.SectionStatusPending,
			CreateBy:        &by,
			UpdateBy:        &by,
			CreateAt:        now,
			UpdateAt:        now,
		}
		// 大纲阶段确认的环节内容描述 → 内容生成的固化要求（生成器 user 模板的描述段）。
		if desc := strings.TrimSpace(os.Description); desc != "" {
			row.Prompt = &desc
		}
		rows = append(rows, row)
	}

	// 同一事务：覆盖大纲（终态有序列表 + confirmed）+ 课程状态 + 物化环节。
	err = s.tm.Transaction(ctx, func(ctx context.Context) error {
		if err := s.outlineRepo.UpdateContentStatus(ctx, courseID, content, types.OutlineStatusConfirmed, &by); err != nil {
			return err
		}
		if err := s.courseRepo.UpdateStatus(ctx, courseID, types.CourseStatusOutlineConfirmed); err != nil {
			return err
		}
		return s.sectionRepo.CreateBulk(ctx, rows)
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后再启动后台串行生成。
	s.ensureLoop(userID, course)

	progs := make([]types.SectionProgress, 0, len(rows))
	for i := range rows {
		progs = append(progs, sectionToProgress(&rows[i]))
	}
	return progs, nil
}

// ---- 进度 ----

func (s *SectionService) ListProgress(ctx context.Context, userID, courseID types.ID) ([]types.SectionProgress, error) {
	if _, err := s.courseRepo.GetByID(ctx, userID, courseID); err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}
	return s.listProgress(ctx, courseID)
}

// listProgress 免鉴权读取（调用方需已校验归属）。
func (s *SectionService) listProgress(ctx context.Context, courseID types.ID) ([]types.SectionProgress, error) {
	secs, err := s.sectionRepo.ListByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	out := make([]types.SectionProgress, 0, len(secs))
	for i := range secs {
		out = append(out, sectionToProgress(&secs[i]))
	}
	return out, nil
}

// GetLearnDetail 返回课程学习详情。仅课程 owner 可访问；slide 环节的
// content / steps 产物以原始 JSON 透传，quiz 环节加载 question 表题目，demo 环节为占位产物。
func (s *SectionService) GetLearnDetail(ctx context.Context, userID, courseID types.ID) (*types.CourseLearnDetail, error) {
	course, err := s.courseRepo.GetByID(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}
	secs, err := s.sectionRepo.ListByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	out := make([]types.SectionLearn, 0, len(secs))
	for i := range secs {
		sec := &secs[i]
		sl := types.SectionLearn{
			ID:              sec.ID,
			Position:        sec.Position,
			Type:            sec.Type,
			Title:           sec.Title,
			KnowledgePoints: unmarshalKP(sec.KnowledgePoints),
			Status:          sec.Status,
			Content:         json.RawMessage(sec.Content),
			Steps:           json.RawMessage(sec.Steps),
			Questions:       []types.LearnQuestion{},
		}
		if sec.Type == types.SectionTypeQuiz {
			qs, qerr := s.loadQuestions(ctx, sec.ID)
			if qerr != nil {
				return nil, qerr
			}
			sl.Questions = qs
		}
		out = append(out, sl)
	}

	return &types.CourseLearnDetail{
		Course:   types.LearnCourse{ID: course.ID, Title: course.Title},
		Progress: types.ProgressStatusUnstarted,
		Sections: out,
	}, nil
}

// loadQuestions 加载某 quiz 环节题目并转为学习 DTO。
func (s *SectionService) loadQuestions(ctx context.Context, sectionID types.ID) ([]types.LearnQuestion, error) {
	qs, err := s.questionRepo.ListBySection(ctx, sectionID)
	if err != nil {
		return nil, err
	}
	out := make([]types.LearnQuestion, 0, len(qs))
	for i := range qs {
		q := &qs[i]
		out = append(out, types.LearnQuestion{
			ID:           q.ID,
			Position:     q.Position,
			Type:         q.Type,
			Stem:         q.Stem,
			Options:      unmarshalStringSlice(q.Options),
			Answers:      unmarshalIntSlice(q.Answers),
			Explanations: unmarshalStringSlice(q.Explanations),
		})
	}
	return out, nil
}

// ---- 串行生成 ----

// EnsureGeneration 确保某课程的内容生成循环在运行（未运行则启动/续跑）。
// 用于进程重启或中断后的恢复。
func (s *SectionService) EnsureGeneration(ctx context.Context, userID, courseID types.ID) error {
	course, err := s.courseRepo.GetByID(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return types.ErrCourseNotFound
		}
		return err
	}
	switch course.Status {
	case types.CourseStatusGenerating, types.CourseStatusOutlineConfirmed:
	default:
		return types.ErrOutlineNotConfirmed
	}
	s.ensureLoop(userID, course)
	return nil
}

// ensureLoop 保证单课程只跑一个生成循环；如未在跑则启动（含崩溃后恢复续跑）。
func (s *SectionService) ensureLoop(userID types.ID, course *types.Course) {
	if !s.runs.tryStart(course.ID) {
		return
	}
	go s.runGeneration(userID, course)
}

// runGeneration 串行生成课程全部未完成环节。任何一处失败即中止：失败环节置回 pending，
// 课程停留 generating；前端据此展示「重试继续生成」并经 EnsureGeneration 续跑。
func (s *SectionService) runGeneration(userID types.ID, course *types.Course) {
	ctx := context.Background()
	defer s.runs.finish(course.ID)

	genCtx, err := s.buildGenerationContext(ctx, userID, course)
	if err != nil {
		slog.Error("build generation context failed", "course_id", course.ID.String(), "error", err)
		return
	}

	if err := s.courseRepo.UpdateStatus(ctx, course.ID, types.CourseStatusGenerating); err != nil {
		slog.Error("update course status failed", "course_id", course.ID.String(), "error", err)
		return
	}

	secs, err := s.sectionRepo.ListByCourse(ctx, course.ID)
	if err != nil {
		slog.Error("list sections failed", "course_id", course.ID.String(), "error", err)
		return
	}

	for i := range secs {
		sec := &secs[i]
		if sec.Status == types.SectionStatusDone {
			genCtx.Done = append(genCtx.Done, *sec)
			continue
		}
		if err := s.sectionRepo.UpdateStatus(ctx, sec.ID, types.SectionStatusGenerating); err != nil {
			slog.Error("update section status failed", "section_id", sec.ID.String(), "error", err)
			return
		}

		gen := s.generators[sec.Type]
		if gen == nil {
			gen = s.generators[types.SectionTypeSlide]
		}
		if err := gen.Generate(ctx, sec, &genCtx); err != nil {
			slog.Error("generate section failed", "section_id", sec.ID.String(), "error", err)
			_ = s.sectionRepo.UpdateStatus(ctx, sec.ID, types.SectionStatusPending)
			return
		}
		if err := s.sectionRepo.UpdateContentSteps(ctx, sec.ID, sec.Content, sec.Steps); err != nil {
			slog.Error("persist section output failed", "section_id", sec.ID.String(), "error", err)
			return
		}
		if err := s.sectionRepo.UpdateStatus(ctx, sec.ID, types.SectionStatusDone); err != nil {
			slog.Error("update section status failed", "section_id", sec.ID.String(), "error", err)
			return
		}
		genCtx.Done = append(genCtx.Done, *sec)
	}

	if err := s.courseRepo.UpdateStatus(ctx, course.ID, types.CourseStatusCompleted); err != nil {
		slog.Error("update course status failed", "course_id", course.ID.String(), "error", err)
	}
}

// buildGenerationContext 解析模型配置、构造客户端、加载文档与全量大纲，
// 组装传给各环节内容生成器的上下文。
func (s *SectionService) buildGenerationContext(ctx context.Context, userID types.ID, course *types.Course) (types.GenerationContext, error) {
	var cfg model.ProviderConfig
	var err error
	if course.ModelConfigID != nil && *course.ModelConfigID != types.NilID {
		cfg, err = s.modelCfgSvc.ResolveByID(ctx, userID, *course.ModelConfigID)
	} else {
		cfg, err = s.modelCfgSvc.ResolveDefault(ctx, userID)
	}
	if err != nil {
		return types.GenerationContext{}, err
	}
	if cfg.Model == "" || cfg.APIKey == "" {
		return types.GenerationContext{}, types.ErrNoModelConfig
	}
	client := s.registry.NewLLM(cfg)
	if client == nil {
		return types.GenerationContext{}, types.ErrNoModelConfig
	}

	// 文生图为可选能力：课程关闭配图时不产图；开启时优先用绑定配置，否则跟随用户默认 image 配置。
	var imageClient model.ImageClient
	if course.GenerateImages {
		var imgCfg model.ProviderConfig
		var ierr error
		if course.ImageModelConfigID != nil && *course.ImageModelConfigID != types.NilID {
			imgCfg, ierr = s.modelCfgSvc.ResolveByID(ctx, userID, *course.ImageModelConfigID)
		} else {
			imgCfg, ierr = s.modelCfgSvc.ResolveDefaultByKind(ctx, userID, types.ModelKindImage)
		}
		if ierr == nil && imgCfg.Model != "" && imgCfg.APIKey != "" {
			imageClient = s.registry.NewImage(imgCfg)
		}
	}

	secs, err := s.sectionRepo.ListByCourse(ctx, course.ID)
	if err != nil {
		return types.GenerationContext{}, err
	}
	docsText, err := loadDocumentsText(ctx, s.docRepo, s.storage, course.ID)
	if err != nil {
		return types.GenerationContext{}, err
	}

	return types.GenerationContext{
		Course:          course,
		OutlineSections: sectionsToOutline(secs),
		DocsText:        docsText,
		Client:          client,
		ImageClient:     imageClient,
		Storage:         s.storage,
		Thinking:        course.Thinking,
	}, nil
}

// sectionsToOutline 将物化环节转换为大纲结构（标题/类型/知识点），供连贯性参考。
func sectionsToOutline(secs []types.Section) []types.OutlineSection {
	out := make([]types.OutlineSection, 0, len(secs))
	for i := range secs {
		kp := make([]string, 0)
		_ = json.Unmarshal(secs[i].KnowledgePoints, &kp)
		out = append(out, types.OutlineSection{
			Title:           secs[i].Title,
			Type:            secs[i].Type,
			KnowledgePoints: kp,
		})
	}
	return out
}

// ---- helpers ----

// unmarshalKP 将 jsonb 的知识点列解析为字符串切片；解析失败返回空切片。
func unmarshalKP(raw datatypes.JSON) []string {
	kp := make([]string, 0)
	_ = json.Unmarshal(raw, &kp)
	return kp
}

// unmarshalStringSlice 将 jsonb 字符串数组列解析为切片；解析失败返回空切片。
func unmarshalStringSlice(raw datatypes.JSON) []string {
	out := make([]string, 0)
	_ = json.Unmarshal(raw, &out)
	return out
}

// unmarshalIntSlice 将 jsonb 整数数组列解析为切片；解析失败返回空切片。
func unmarshalIntSlice(raw datatypes.JSON) []int {
	out := make([]int, 0)
	_ = json.Unmarshal(raw, &out)
	return out
}

// sectionToProgress 将实体转为进度 DTO（解出知识点列表）。
func sectionToProgress(s *types.Section) types.SectionProgress {
	kp := unmarshalKP(s.KnowledgePoints)
	return types.SectionProgress{
		ID:              s.ID,
		Position:        s.Position,
		Type:            s.Type,
		Title:           s.Title,
		Status:          s.Status,
		KnowledgePoints: kp,
	}
}
