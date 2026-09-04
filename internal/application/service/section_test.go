package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

type mockSectionRepo struct {
	createBulk func([]types.Section) error
	listBy     func(types.ID) ([]types.Section, error)
	updateSt   func(types.ID, string) error
	updateCS   func(types.ID, datatypes.JSON, datatypes.JSON) error
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
	testSlideContentJSON = `{"width":1280,"height":720,"background":"#ffffff","accent":"#14b8a6","elements":[{"id":"e1","type":"text","content":"数组的定义"},{"id":"e2","type":"shape","shape":"rect","label":"arr[0]"}]}`
	testSlideStepsJSON   = `[{"text":"数组是……","actions":[{"type":"highlight","targetElementId":"e1"}]},{"text":"看第一个元素。","actions":[{"type":"box","targetElementId":"e2"}]}]`
)

func newSectionSvc(course types.CourseRepo, outline types.OutlineRepo, sec types.SectionRepo) types.SectionService {
	return NewSectionService(course, outline, sec, passTM{}, &mockDocRepo{}, &mockStorage{}, &mockModelCfgSvc{
		resolve: func() (model.ProviderConfig, error) {
			return model.ProviderConfig{Provider: "test", Model: "m", APIKey: "k"}, nil
		},
	}, testRegistry())
}

// testRegistry 注册 test provider，返回按次产出 content/steps 的 Slide LLM。
func testRegistry() *model.Registry {
	registry := model.NewRegistry()
	registry.RegisterLLM("test", func(model.ProviderConfig) model.LLMClient {
		return &seqLLM{contents: []string{testSlideContentJSON, testSlideStepsJSON}}
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
	_, err := svc.ConfirmOutline(context.Background(), uid, cid, &types.ConfirmOutlineReq{
		Sections: []types.OutlineSection{{Title: "x", Type: types.SectionTypeSlide}},
	})
	require.ErrorIs(t, err, types.ErrOutlineAlreadyConfirmed)
}

func TestConfirmOutlineEmpty(t *testing.T) {
	uid := types.NewID()
	cid := types.NewID()
	svc := newSectionSvc(&mockCourseRepo{}, &mockOutlineRepo{}, &mockSectionRepo{})
	_, err := svc.ConfirmOutline(context.Background(), uid, cid, &types.ConfirmOutlineReq{})
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

	progs, err := svc.ConfirmOutline(context.Background(), uid, cid, &types.ConfirmOutlineReq{
		Sections: []types.OutlineSection{
			{Title: "声明", Type: types.SectionTypeSlide, KnowledgePoints: []string{"语法"}},
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
	require.Equal(t, &uid, created[0].CreateBy)
	var kp []string
	require.NoError(t, json.Unmarshal(created[0].KnowledgePoints, &kp))
	require.Equal(t, []string{"语法"}, kp)
	require.Len(t, progs, 2)
}

func TestStreamGenerationSerialCompletion(t *testing.T) {
	uid := types.NewID()
	cid := types.NewID()
	kpJSON, _ := json.Marshal([]string{"a"})

	sections := []types.Section{
		{ID: types.NewID(), CourseID: cid, Position: 1, Type: types.SectionTypeSlide, Title: "s1", KnowledgePoints: kpJSON, Status: types.SectionStatusPending, CreateAt: time.Now(), UpdateAt: time.Now()},
		{ID: types.NewID(), CourseID: cid, Position: 2, Type: types.SectionTypeQuiz, Title: "s2", KnowledgePoints: kpJSON, Status: types.SectionStatusPending, CreateAt: time.Now(), UpdateAt: time.Now()},
	}

	var completedStatus string
	svc := newSectionSvc(
		&mockCourseRepo{
			getByID: func(_, _ types.ID) (*types.Course, error) {
				c := sampleCourse(cid, uid, types.CourseStatusOutlineConfirmed, false)
				return &c, nil
			},
			updateSt: func(_ types.ID, st string) error { completedStatus = st; return nil },
		},
		&mockOutlineRepo{},
		&mockSectionRepo{
			listBy: func(_ types.ID) ([]types.Section, error) {
				// 每次返回独立副本，避免与生成循环的并发写入产生竞争。
				cp := make([]types.Section, len(sections))
				copy(cp, sections)
				return cp, nil
			},
		},
	)

	var events []types.ProgressEvent
	done := make(chan error, 1)
	go func() {
		err := svc.StreamGeneration(context.Background(), uid, cid, func(ev types.ProgressEvent) error {
			events = append(events, ev)
			return nil
		})
		done <- err
	}()

	require.NoError(t, <-done)
	require.Equal(t, types.CourseStatusCompleted, completedStatus)
	require.NotEmpty(t, events)
	require.Equal(t, "snapshot", events[0].Type)

	doneSec := 0
	for _, ev := range events {
		if ev.Type == "section" && ev.Section.Status == types.SectionStatusDone {
			doneSec++
		}
	}
	require.Equal(t, 2, doneSec, "两个环节都应串行完成")
}
