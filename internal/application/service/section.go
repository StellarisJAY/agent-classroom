package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

// ---- 进程内进度 hub（简化实现：无分布式 worker，单进程内存广播） ----

// eventBuffer 订阅者缓冲事件数；生成中事件较多，缓冲防阻塞。
const eventBuffer = 16

// courseFeed 单个课程的订阅与终态。
type courseFeed struct {
	mu      sync.Mutex
	running bool
	done    bool
	subs    map[chan types.ProgressEvent]struct{}
}

type progressHub struct {
	mu    sync.Mutex
	feeds map[types.ID]*courseFeed
}

func newProgressHub() *progressHub {
	return &progressHub{feeds: make(map[types.ID]*courseFeed)}
}

func (h *progressHub) feed(courseID types.ID) *courseFeed {
	h.mu.Lock()
	defer h.mu.Unlock()
	f, ok := h.feeds[courseID]
	if !ok {
		f = &courseFeed{subs: make(map[chan types.ProgressEvent]struct{})}
		h.feeds[courseID] = f
	}
	return f
}

func (f *courseFeed) add() chan types.ProgressEvent {
	ch := make(chan types.ProgressEvent, eventBuffer)
	f.mu.Lock()
	f.subs[ch] = struct{}{}
	f.mu.Unlock()
	return ch
}

func (f *courseFeed) remove(ch chan types.ProgressEvent) {
	f.mu.Lock()
	delete(f.subs, ch)
	f.mu.Unlock()
}

// tryStart 返回是否成功抢占运行权（保证单进程内单课程只跑一个串行循环）。
func (f *courseFeed) tryStart() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.running {
		return false
	}
	f.running = true
	return true
}

func (f *courseFeed) isDone() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.done
}

// broadcast 非阻塞广播给所有订阅者。
func (f *courseFeed) broadcast(ev types.ProgressEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for ch := range f.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// finish 标记运行结束；调用前须已广播终态事件。
func (f *courseFeed) finish() {
	f.mu.Lock()
	f.running = false
	f.done = true
	f.mu.Unlock()
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
	hub          *progressHub
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
		hub: newProgressHub(),
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
		rows = append(rows, types.Section{
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
		})
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

// ---- SSE 进度订阅 ----

func (s *SectionService) StreamGeneration(ctx context.Context, userID, courseID types.ID, emit func(types.ProgressEvent) error) error {
	course, err := s.courseRepo.GetByID(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return types.ErrCourseNotFound
		}
		return err
	}
	switch course.Status {
	case types.CourseStatusCompleted:
		if secs, lerr := s.listProgress(ctx, courseID); lerr == nil {
			if eerr := emit(types.ProgressEvent{Type: "snapshot", Sections: secs}); eerr != nil {
				return nil
			}
		}
		return nil
	case types.CourseStatusGenerating, types.CourseStatusOutlineConfirmed:
	default:
		return types.ErrOutlineNotConfirmed
	}

	f := s.hub.feed(courseID)
	ch := f.add()
	defer f.remove(ch)

	// 已结束（本地刚完成）：快照即含全部 done，直接返回终态。
	if f.isDone() {
		if secs, lerr := s.listProgress(ctx, courseID); lerr == nil {
			_ = emit(types.ProgressEvent{Type: "snapshot", Sections: secs})
		}
		return nil
	}

	// 先订阅再启动/恢复循环，避免漏事件。
	s.ensureLoop(userID, course)

	// 快照：先让前端渲染当前全部进度，后续事件按 position 增量更新。
	if secs, lerr := s.listProgress(ctx, courseID); lerr == nil {
		if eerr := emit(types.ProgressEvent{Type: "snapshot", Sections: secs}); eerr != nil {
			return nil
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			switch ev.Type {
			case "error":
				return types.NewError(types.CodeInternalError, ev.Message)
			case "course":
				// 终态；返回后由 handler 写 done
				return nil
			}
			if err := emit(ev); err != nil {
				return nil
			}
		}
	}
}

// ---- 串行生成 ----

// ensureLoop 保证单课程只跑一个生成循环；如未在跑则启动（含崩溃后恢复续跑）。
func (s *SectionService) ensureLoop(userID types.ID, course *types.Course) {
	f := s.hub.feed(course.ID)
	if !f.tryStart() {
		return
	}
	go s.runGeneration(userID, course, f)
}

// runGeneration 串行生成课程全部未完成环节。任何一处失败即中止并广播 error。
func (s *SectionService) runGeneration(userID types.ID, course *types.Course, f *courseFeed) {
	ctx := context.Background()
	defer f.finish()

	genCtx, err := s.buildGenerationContext(ctx, userID, course)
	if err != nil {
		slog.Error("build generation context failed", "course_id", course.ID.String(), "error", err)
		f.broadcast(types.ProgressEvent{Type: "error", Message: types.ErrContentGenerate.Msg})
		return
	}

	if err := s.courseRepo.UpdateStatus(ctx, course.ID, types.CourseStatusGenerating); err != nil {
		f.broadcast(types.ProgressEvent{Type: "error", Message: types.ErrContentGenerate.Msg})
		return
	}

	secs, err := s.sectionRepo.ListByCourse(ctx, course.ID)
	if err != nil {
		f.broadcast(types.ProgressEvent{Type: "error", Message: types.ErrContentGenerate.Msg})
		return
	}

	for i := range secs {
		sec := &secs[i]
		if sec.Status == types.SectionStatusDone {
			genCtx.Done = append(genCtx.Done, *sec)
			continue
		}
		if err := s.sectionRepo.UpdateStatus(ctx, sec.ID, types.SectionStatusGenerating); err != nil {
			f.broadcast(types.ProgressEvent{Type: "error", Message: types.ErrContentGenerate.Msg})
			return
		}
		sp := sectionToProgress(sec)
		sp.Status = types.SectionStatusGenerating
		f.broadcast(types.ProgressEvent{Type: "section", Index: i, Section: sp})

		gen := s.generators[sec.Type]
		if gen == nil {
			gen = s.generators[types.SectionTypeSlide]
		}
		if err := gen.Generate(ctx, sec, &genCtx); err != nil {
			slog.Error("generate section failed", "section_id", sec.ID.String(), "error", err)
			_ = s.sectionRepo.UpdateStatus(ctx, sec.ID, types.SectionStatusPending)
			f.broadcast(types.ProgressEvent{Type: "error", Message: types.ErrContentGenerate.Msg})
			return
		}
		if err := s.sectionRepo.UpdateContentSteps(ctx, sec.ID, sec.Content, sec.Steps); err != nil {
			f.broadcast(types.ProgressEvent{Type: "error", Message: types.ErrContentGenerate.Msg})
			return
		}
		if err := s.sectionRepo.UpdateStatus(ctx, sec.ID, types.SectionStatusDone); err != nil {
			f.broadcast(types.ProgressEvent{Type: "error", Message: types.ErrContentGenerate.Msg})
			return
		}
		sp.Status = types.SectionStatusDone
		f.broadcast(types.ProgressEvent{Type: "section", Index: i, Section: sp})
		genCtx.Done = append(genCtx.Done, *sec)
	}

	if err := s.courseRepo.UpdateStatus(ctx, course.ID, types.CourseStatusCompleted); err != nil {
		f.broadcast(types.ProgressEvent{Type: "error", Message: types.ErrContentGenerate.Msg})
		return
	}
	f.broadcast(types.ProgressEvent{Type: "course", Status: types.CourseStatusCompleted})
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
