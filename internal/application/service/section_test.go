package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/config"
	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

type mockSectionRepo struct {
	createBulk func([]types.Section) error
	listBy     func(types.ID) ([]types.Section, error)
	updateSt   func(types.ID, string) error
	updateCS   func(types.ID, datatypes.JSON, datatypes.JSON) error
	updateFail func(types.ID, string) error
}

var _ types.SectionRepo = (*mockSectionRepo)(nil)

func (m *mockSectionRepo) CreateBulk(_ context.Context, secs []types.Section) error {
	if m.createBulk != nil {
		return m.createBulk(secs)
	}
	return nil
}
func (m *mockSectionRepo) ListByCourse(_ context.Context, courseID types.ID) ([]types.Section, error) {
	if m.listBy != nil {
		return m.listBy(courseID)
	}
	return nil, nil
}
func (m *mockSectionRepo) UpdateStatus(_ context.Context, id types.ID, status string) error {
	if m.updateSt != nil {
		return m.updateSt(id, status)
	}
	return nil
}
func (m *mockSectionRepo) UpdateContentSteps(_ context.Context, id types.ID, content, steps datatypes.JSON) error {
	if m.updateCS != nil {
		return m.updateCS(id, content, steps)
	}
	return nil
}

func (m *mockSectionRepo) UpdateFailure(_ context.Context, id types.ID, reason string) error {
	if m.updateFail != nil {
		return m.updateFail(id, reason)
	}
	return nil
}

type mockQuestionRepo struct {
	replaceBy func(types.ID, []types.Question) error
	listBy    func(types.ID) ([]types.Question, error)
}

var _ types.QuestionRepo = (*mockQuestionRepo)(nil)

func (m *mockQuestionRepo) ReplaceBySection(_ context.Context, sectionID types.ID, qs []types.Question) error {
	if m.replaceBy != nil {
		return m.replaceBy(sectionID, qs)
	}
	return nil
}
func (m *mockQuestionRepo) ListBySection(_ context.Context, sectionID types.ID) ([]types.Question, error) {
	if m.listBy != nil {
		return m.listBy(sectionID)
	}
	return nil, nil
}

// seqLLM 按调用次序返回内容：阶段一(content JSON)、阶段二(steps JSON)，用于 Slide 真生成路径。
type seqLLM struct {
	mu       sync.Mutex
	contents []string
	err      error
}

func (f *seqLLM) Chat(_ context.Context, _ model.ChatRequest) (*model.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if len(f.contents) == 0 {
		return &model.ChatResponse{Content: `{"width":1280,"height":720,"background":"#fff","accent":"#14b8a6","elements":[{"id":"e1","type":"text","content":"标题"}]}`}, nil
	}
	c := f.contents[0]
	f.contents = f.contents[1:]
	return &model.ChatResponse{Content: c}, nil
}

func (f *seqLLM) ChatStream(context.Context, model.ChatRequest, model.StreamCallback) error {
	return nil
}

const (
	testSlideContentJSON  = `{"width":1280,"height":720,"background":"#ffffff","accent":"#14b8a6","elements":[{"id":"e1","type":"text","content":"数组的定义"},{"id":"e2","type":"shape","shape":"rect","label":"arr[0]"}]}`
	testSlideStepsJSON    = `[{"text":"数组是……","actions":[{"type":"highlight","targetElementId":"e1"}]},{"text":"看第一个元素。","actions":[{"type":"box","targetElementId":"e2"}]}]`
	testQuizQuestionsJSON = `[{"type":"single","stem":"数组下标从几开始？","options":["0","1"],"answers":[0],"explanations":["下标从 0 开始","错误"]},{"type":"multiple","stem":"数组特点？","options":["连续","同类型","长度可变"],"answers":[0,1],"explanations":["正确","正确","错误"]}]`
)

func newSectionSvc(course types.CourseRepo, outline types.OutlineRepo, sec types.SectionRepo) types.SectionService {
	return NewSectionService(course, outline, sec, &mockQuestionRepo{}, passTM{}, &mockDocRepo{}, &mockStorage{}, &mockModelCfgSvc{
		resolve: func() (model.ProviderConfig, error) {
			return model.ProviderConfig{Provider: "test", Model: "m", APIKey: "k"}, nil
		},
	}, testRegistry(), newTestDocsMock(), &config.Config{})
}

// newSectionSvcFull 带自定义 questionRepo 与注册表构造，供走真实生成（含 quiz）的测试使用。
func newSectionSvcFull(course types.CourseRepo, outline types.OutlineRepo, sec types.SectionRepo, question types.QuestionRepo, registry *model.Registry) types.SectionService {
	return NewSectionService(course, outline, sec, question, passTM{}, &mockDocRepo{}, &mockStorage{}, &mockModelCfgSvc{
		resolve: func() (model.ProviderConfig, error) {
			return model.ProviderConfig{Provider: "test", Model: "m", APIKey: "k"}, nil
		},
	}, registry, newTestDocsMock(), &config.Config{})
}

// testRegistry 注册 test provider，按序产出给定 LLM 响应（缺省 Slide content/steps）。
func testRegistry(contents ...string) *model.Registry {
	if len(contents) == 0 {
		contents = []string{testSlideContentJSON, testSlideStepsJSON}
	}
	registry := model.NewRegistry()
	registry.RegisterLLM("test", func(model.ProviderConfig) model.LLMClient {
		return &seqLLM{contents: contents}
	})
	return registry
}

func TestConfirmOutlineRejectsNonDraft(t *testing.T) {
	uid := types.NewID()
	cid := types.NewID()
	svc := newSectionSvc(
		&mockCourseRepo{getByID: func(_, _ types.ID) (*types.Course, error) {
			c := sampleCourse(cid, uid, types.CourseStatusGenerating, false)
			return &c, nil
		}},
		&mockOutlineRepo{},
		&mockSectionRepo{},
	)
	_, err := svc.ConfirmOutline(context.Background(), uid, cid, types.ConfirmOutlineReq{
		Sections: []types.OutlineSection{{Title: "x", Type: types.SectionTypeSlide}},
	})
	require.ErrorIs(t, err, types.ErrOutlineAlreadyConfirmed)
}

func TestConfirmOutlineEmpty(t *testing.T) {
	uid := types.NewID()
	cid := types.NewID()
	svc := newSectionSvc(&mockCourseRepo{}, &mockOutlineRepo{}, &mockSectionRepo{})
	_, err := svc.ConfirmOutline(context.Background(), uid, cid, types.ConfirmOutlineReq{})
	require.ErrorIs(t, err, types.ErrInvalidRequest)
}

func TestConfirmOutlineMaterializesSections(t *testing.T) {
	uid := types.NewID()
	cid := types.NewID()

	var mu sync.Mutex
	var courseStatus []string
	var created []types.Section
	var outlineStatus string

	svc := newSectionSvc(
		&mockCourseRepo{
			getByID: func(_, _ types.ID) (*types.Course, error) {
				c := sampleCourse(cid, uid, types.CourseStatusDraft, false)
				return &c, nil
			},
			updateSt: func(_ types.ID, st string) error {
				mu.Lock()
				courseStatus = append(courseStatus, st)
				mu.Unlock()
				return nil
			},
		},
		&mockOutlineRepo{
			getBy: func(_ types.ID) (*types.Outline, error) {
				return &types.Outline{CourseID: cid, Status: types.OutlineStatusDraft}, nil
			},
			update: func(_ types.ID, _ datatypes.JSON, st string, _ *types.ID) error {
				outlineStatus = st
				return nil
			},
		},
		&mockSectionRepo{
			createBulk: func(secs []types.Section) error { created = secs; return nil },
		},
	)

	progs, err := svc.ConfirmOutline(context.Background(), uid, cid, types.ConfirmOutlineReq{
		Sections: []types.OutlineSection{
			{Title: "声明", Type: types.SectionTypeSlide, KnowledgePoints: []string{"语法"}, Description: "先讲定义，再给类比"},
			{Title: "访问", Type: types.SectionTypeQuiz, KnowledgePoints: []string{"索引"}},
		},
	})
	require.NoError(t, err)
	mu.Lock()
	require.Contains(t, courseStatus, types.CourseStatusOutlineConfirmed)
	mu.Unlock()
	require.Equal(t, types.OutlineStatusConfirmed, outlineStatus)
	require.Len(t, created, 2)
	require.Equal(t, 1, created[0].Position)
	require.Equal(t, 2, created[1].Position)
	require.Equal(t, types.SectionStatusPending, created[0].Status)
	require.Equal(t, types.SectionTypeSlide, created[0].Type)
	require.Equal(t, "声明", created[0].Title)
	require.Equal(t, "先讲定义，再给类比", *created[0].Prompt, "大纲描述应物化为环节内容要求")
	require.Nil(t, created[1].Prompt, "无描述的环节不应设置内容要求")
	require.Equal(t, &uid, created[0].CreateBy)
	var kp []string
	require.NoError(t, json.Unmarshal(created[0].KnowledgePoints, &kp))
	require.Equal(t, []string{"语法"}, kp)
	require.Len(t, progs, 2)
}

func TestEnsureGenerationSerialCompletion(t *testing.T) {
	uid := types.NewID()
	cid := types.NewID()
	kpJSON, _ := json.Marshal([]string{"a"})

	sections := []types.Section{
		{ID: types.NewID(), CourseID: cid, Position: 1, Type: types.SectionTypeSlide, Title: "s1", KnowledgePoints: kpJSON, Status: types.SectionStatusPending, CreateAt: time.Now(), UpdateAt: time.Now()},
		{ID: types.NewID(), CourseID: cid, Position: 2, Type: types.SectionTypeQuiz, Title: "s2", KnowledgePoints: kpJSON, Status: types.SectionStatusPending, CreateAt: time.Now(), UpdateAt: time.Now()},
	}

	var completedStatus string
	var replacedBy map[types.ID][]types.Question
	var qMu sync.Mutex
	svc := newSectionSvcFull(
		&mockCourseRepo{
			getByID: func(_, _ types.ID) (*types.Course, error) {
				c := sampleCourse(cid, uid, types.CourseStatusOutlineConfirmed, false)
				return &c, nil
			},
			updateSt: func(_ types.ID, st string) error {
				qMu.Lock()
				completedStatus = st
				qMu.Unlock()
				return nil
			},
		},
		&mockOutlineRepo{},
		&mockSectionRepo{
			listBy: func(_ types.ID) ([]types.Section, error) {
				// 每次返回独立副本，避免与生成循环的并发写入产生竞争。
				cp := make([]types.Section, len(sections))
				copy(cp, sections)
				return cp, nil
			},
			// 状态落库同步回来源切片，模拟 DB 状态演进（evaluateCourseStatus 依赖）。
			updateSt: func(id types.ID, st string) error {
				qMu.Lock()
				defer qMu.Unlock()
				for i := range sections {
					if sections[i].ID == id {
						sections[i].Status = st
					}
				}
				return nil
			},
		},
		&mockQuestionRepo{
			replaceBy: func(sectionID types.ID, qs []types.Question) error {
				qMu.Lock()
				defer qMu.Unlock()
				if replacedBy == nil {
					replacedBy = map[types.ID][]types.Question{}
				}
				replacedBy[sectionID] = qs
				return nil
			},
		},
		testRegistry(testSlideContentJSON, testSlideStepsJSON, testQuizQuestionsJSON),
	)

	// 后台异步运行；轮询等待两环节全部完成。
	require.NoError(t, svc.EnsureGeneration(context.Background(), uid, cid))

	deadline := time.After(5 * time.Second)
	for {
		qMu.Lock()
		finished := completedStatus == types.CourseStatusCompleted && len(replacedBy[sections[1].ID]) == 2
		qMu.Unlock()
		if finished {
			break
		}
		select {
		case <-deadline:
			t.Fatal("内容生成未在预期时间内完成")
		case <-time.After(50 * time.Millisecond):
		}
	}

	qMu.Lock()
	defer qMu.Unlock()
	require.Equal(t, types.CourseStatusCompleted, completedStatus)
	quizSec := sections[1]
	require.Len(t, replacedBy[quizSec.ID], 2, "quiz 环节应把生成题目写入 questionRepo")
}

// waitCourseStatus 轮询等待课程状态演进为 want（后台生成循环异步执行）。
func waitFor(t *testing.T, want func() bool, msg string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if want() {
			return
		}
		select {
		case <-deadline:
			t.Fatal(msg)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// 生成失败（LLM 持续产出非法内容）→ 环节置 failed 并记录原因，不回退 pending；
// 其余环节正常完成 → 课程置 partial_failed；循环不中止。
func TestRunGenerationFailureMarksFailedAndContinues(t *testing.T) {
	uid := types.NewID()
	cid := types.NewID()
	kpJSON, _ := json.Marshal([]string{"a"})

	sections := []types.Section{
		{ID: types.NewID(), CourseID: cid, Position: 1, Type: types.SectionTypeSlide, Title: "ok", KnowledgePoints: kpJSON, Status: types.SectionStatusPending},
		{ID: types.NewID(), CourseID: cid, Position: 2, Type: types.SectionTypeQuiz, Title: "bad", KnowledgePoints: kpJSON, Status: types.SectionStatusPending},
	}
	var mu sync.Mutex
	var courseStatus []string
	var failureReasons map[types.ID]string
	var statusSeq []string

	// 注册表：slide 两阶段正常；quiz 返回非法 JSON（重试仍失败）。
	registry := model.NewRegistry()
	registry.RegisterLLM("test", func(model.ProviderConfig) model.LLMClient {
		return &seqLLM{contents: []string{testSlideContentJSON, testSlideStepsJSON, "not-json"}}
	})
	svc := newSectionSvcFull(
		&mockCourseRepo{
			getByID: func(_, _ types.ID) (*types.Course, error) {
				c := sampleCourse(cid, uid, types.CourseStatusOutlineConfirmed, false)
				return &c, nil
			},
			updateSt: func(_ types.ID, st string) error {
				mu.Lock()
				courseStatus = append(courseStatus, st)
				mu.Unlock()
				return nil
			},
		},
		&mockOutlineRepo{},
		&mockSectionRepo{
			listBy: func(_ types.ID) ([]types.Section, error) {
				mu.Lock()
				cp := make([]types.Section, len(sections))
				copy(cp, sections)
				mu.Unlock()
				return cp, nil
			},
			updateSt: func(id types.ID, st string) error {
				mu.Lock()
				defer mu.Unlock()
				statusSeq = append(statusSeq, st)
				for i := range sections {
					if sections[i].ID == id {
						sections[i].Status = st
					}
				}
				return nil
			},
			updateFail: func(id types.ID, reason string) error {
				mu.Lock()
				defer mu.Unlock()
				statusSeq = append(statusSeq, types.SectionStatusFailed)
				if failureReasons == nil {
					failureReasons = map[types.ID]string{}
				}
				failureReasons[id] = reason
				for i := range sections {
					if sections[i].ID == id {
						sections[i].Status = types.SectionStatusFailed
					}
				}
				return nil
			},
		},
		&mockQuestionRepo{},
		registry,
	)
	require.NoError(t, svc.EnsureGeneration(context.Background(), uid, cid))

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(courseStatus) > 0 && courseStatus[len(courseStatus)-1] == types.CourseStatusPartialFailed
	}, "生成循环未在预期时间内演进到 partial_failed")

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, types.CourseStatusPartialFailed, courseStatus[len(courseStatus)-1])
	require.True(t, statusSeqContains(statusSeq, types.SectionStatusFailed), "失败环节应置 failed")
	require.NotEmpty(t, failureReasons[sections[1].ID], "失败原因应入库供排查")
	require.Equal(t, types.SectionStatusDone, sections[0].Status, "未失败环节应正常完成，循环不中止")
}

// statusSeqContains 判断状态序列中出现过指定状态。
func statusSeqContains(seq []string, st string) bool {
	for _, v := range seq {
		if v == st {
			return true
		}
	}
	return false
}

// RetrySection 约束与成功路径：
//   - 运行权被占用时重试被拒绝（须等本轮生成结束）；
//   - 非 failed 环节重试被拒绝；
//   - failed 环节重试成功后课程状态重评（partial_failed → completed）。
func TestRetrySection(t *testing.T) {
	uid := types.NewID()
	cid := types.NewID()
	sid := types.NewID()

	sections := []types.Section{
		{ID: types.NewID(), CourseID: cid, Position: 1, Type: types.SectionTypeSlide, Title: "ok", Status: types.SectionStatusDone},
		{ID: sid, CourseID: cid, Position: 2, Type: types.SectionTypeQuiz, Title: "bad", Status: types.SectionStatusFailed},
	}
	var mu sync.Mutex
	courseStatus := types.CourseStatusPartialFailed
	var failReasons map[types.ID]string
	var statuses []string

	svc := newSectionSvcFull(
		&mockCourseRepo{
			getByID: func(_, _ types.ID) (*types.Course, error) {
				mu.Lock()
				defer mu.Unlock()
				c := sampleCourse(cid, uid, courseStatus, false)
				return &c, nil
			},
			updateSt: func(_ types.ID, st string) error {
				mu.Lock()
				defer mu.Unlock()
				courseStatus = st
				return nil
			},
		},
		&mockOutlineRepo{},
		&mockSectionRepo{
			listBy: func(_ types.ID) ([]types.Section, error) {
				mu.Lock()
				defer mu.Unlock()
				cp := make([]types.Section, len(sections))
				copy(cp, sections)
				return cp, nil
			},
			updateSt: func(id types.ID, st string) error {
				mu.Lock()
				defer mu.Unlock()
				for i := range sections {
					if sections[i].ID == id {
						sections[i].Status = st
					}
				}
				statuses = append(statuses, st)
				return nil
			},
			updateFail: func(id types.ID, reason string) error {
				mu.Lock()
				defer mu.Unlock()
				if failReasons == nil {
					failReasons = map[types.ID]string{}
				}
				failReasons[id] = reason
				for i := range sections {
					if sections[i].ID == id {
						sections[i].Status = types.SectionStatusFailed
					}
				}
				return nil
			},
		},
		&mockQuestionRepo{},
		testRegistry(testQuizQuestionsJSON),
	)
	svcConcrete := svc.(*SectionService)

	// ① 运行权被占用 → 拒绝重试（须等生成循环结束）。
	require.True(t, svcConcrete.runs.tryStart(cid))
	require.ErrorIs(t, svc.RetrySection(context.Background(), uid, cid, sid), types.ErrGenerationRunning)
	svcConcrete.runs.finish(cid)

	// ② 非 failed 环节 → 拒绝重试。
	require.ErrorIs(t, svc.RetrySection(context.Background(), uid, cid, sections[0].ID), types.ErrSectionNotRetryable)

	// ③ 不存在的环节 → 报错。
	require.ErrorIs(t, svc.RetrySection(context.Background(), uid, cid, types.NewID()), types.ErrSectionNotFound)

	// ④ failed 环节重试成功 → 课程状态重评为 completed。
	require.NoError(t, svc.RetrySection(context.Background(), uid, cid, sid))
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return courseStatus == types.CourseStatusCompleted
	}, "重试后课程状态未在预期时间内重评为 completed")

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, types.CourseStatusCompleted, courseStatus)
	require.Equal(t, types.SectionStatusDone, sections[1].Status)
	require.Empty(t, failReasons[sid], "重试成功后不应保留失败原因")
}
