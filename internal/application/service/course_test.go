package service

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

type mockCourseRepo struct {
	list     func(types.ID, types.CourseListFilter) ([]types.CourseListRow, int64, error)
	create   func(*types.Course) error
	getByID  func(types.ID, types.ID) (*types.Course, error)
	updateTt func(types.ID, string) error
	updateSt func(types.ID, string) error
}

var _ types.CourseRepo = (*mockCourseRepo)(nil)

func (m *mockCourseRepo) List(_ context.Context, userID types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
	return m.list(userID, f)
}
func (m *mockCourseRepo) Create(_ context.Context, c *types.Course) error {
	if c.ID == types.NilID {
		c.ID = types.NewID()
	}
	if m.create != nil {
		return m.create(c)
	}
	return nil
}
func (m *mockCourseRepo) GetByID(_ context.Context, owner, id types.ID) (*types.Course, error) {
	if m.getByID != nil {
		return m.getByID(owner, id)
	}
	return nil, types.ErrNotFound
}
func (m *mockCourseRepo) UpdateTitle(_ context.Context, id types.ID, title string) error {
	if m.updateTt != nil {
		return m.updateTt(id, title)
	}
	return nil
}
func (m *mockCourseRepo) UpdateStatus(_ context.Context, id types.ID, status string) error {
	if m.updateSt != nil {
		return m.updateSt(id, status)
	}
	return nil
}

type mockOutlineRepo struct {
	create   func(*types.Outline) error
	update   func(types.ID, datatypes.JSON, string, *types.ID) error
	getBy    func(types.ID) (*types.Outline, error)
	deleteBy func(types.ID) error
}

var _ types.OutlineRepo = (*mockOutlineRepo)(nil)

func (m *mockOutlineRepo) Create(_ context.Context, o *types.Outline) error {
	if m.create != nil {
		return m.create(o)
	}
	return nil
}
func (m *mockOutlineRepo) UpdateContentStatus(_ context.Context, courseID types.ID, content datatypes.JSON, status string, by *types.ID) error {
	if m.update != nil {
		return m.update(courseID, content, status, by)
	}
	return nil
}
func (m *mockOutlineRepo) GetByCourse(_ context.Context, courseID types.ID) (*types.Outline, error) {
	if m.getBy != nil {
		return m.getBy(courseID)
	}
	return nil, types.ErrNotFound
}
func (m *mockOutlineRepo) DeleteByCourse(_ context.Context, courseID types.ID) error {
	if m.deleteBy != nil {
		return m.deleteBy(courseID)
	}
	return nil
}

type mockDocRepo struct {
	create func(*types.Document) error
	listBy func(types.ID) ([]types.Document, error)
}

var _ types.DocumentRepo = (*mockDocRepo)(nil)

func (m *mockDocRepo) Create(_ context.Context, d *types.Document) error {
	if m.create != nil {
		return m.create(d)
	}
	return nil
}
func (m *mockDocRepo) ListByCourse(_ context.Context, courseID types.ID) ([]types.Document, error) {
	if m.listBy != nil {
		return m.listBy(courseID)
	}
	return nil, nil
}

type mockStorage struct {
	put func(string, io.Reader) (string, error)
	get func(string) (io.ReadCloser, error)
}

var _ types.Storage = (*mockStorage)(nil)

func (m *mockStorage) Put(_ context.Context, key string, r io.Reader) (string, error) {
	if m.put != nil {
		return m.put(key, r)
	}
	return "/uploads/" + key, nil
}
func (m *mockStorage) Get(_ context.Context, url string) (io.ReadCloser, error) {
	if m.get != nil {
		return m.get(url)
	}
	return io.NopCloser(nil), nil
}

// passTM 无操作事务管理器（测试中直接透传）。
type passTM struct{}

func (passTM) Transaction(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type mockModelCfgSvc struct {
	resolve func() (model.ProviderConfig, error)
}

var _ types.ModelConfigService = (*mockModelCfgSvc)(nil)

func (m *mockModelCfgSvc) ResolveDefault(context.Context, types.ID) (model.ProviderConfig, error) {
	return m.resolve()
}
func (m *mockModelCfgSvc) ResolveByID(context.Context, types.ID, types.ID) (model.ProviderConfig, error) {
	return model.ProviderConfig{}, types.ErrNotFound
}
func (m *mockModelCfgSvc) List(context.Context, types.ID) ([]types.ModelConfigInfo, error) {
	return nil, nil
}
func (m *mockModelCfgSvc) Create(context.Context, types.ID, *types.CreateModelConfigReq) (*types.ModelConfigInfo, error) {
	return nil, types.ErrNotFound
}
func (m *mockModelCfgSvc) Update(context.Context, types.ID, types.ID, *types.UpdateModelConfigReq) (*types.ModelConfigInfo, error) {
	return nil, types.ErrNotFound
}
func (m *mockModelCfgSvc) Delete(context.Context, types.ID, types.ID) error {
	return nil
}
func (m *mockModelCfgSvc) SetDefault(context.Context, types.ID, types.ID) error {
	return nil
}

func newCourseSvc(repo types.CourseRepo) types.CourseService {
	set := defaultMockSet()
	set.course = repo
	return NewCourseService(set.course, set.outline, set.doc, passTM{}, set.storage, set.cfgSvc, set.registry)
}

// mockSet 汇总本测试所需各 repo mock。
type mockSet struct {
	course   types.CourseRepo
	outline  types.OutlineRepo
	doc      types.DocumentRepo
	storage  types.Storage
	cfgSvc   types.ModelConfigService
	registry *model.Registry
}

func defaultMockSet() *mockSet {
	return &mockSet{
		course:   &mockCourseRepo{},
		outline:  &mockOutlineRepo{},
		doc:      &mockDocRepo{},
		storage:  &mockStorage{},
		cfgSvc:   &mockModelCfgSvc{resolve: func() (model.ProviderConfig, error) { return model.ProviderConfig{}, types.ErrNotFound }},
		registry: model.NewRegistry(),
	}
}

func sampleCourse(id types.ID, owner types.ID, status string, isPublic bool) types.Course {
	return types.Course{
		ID: id, OwnerID: owner, Title: "数组与循环", Prompt: "学习数组",
		Status: status, IsPublic: isPublic,
		CreateAt: time.Now(), UpdateAt: time.Now(),
	}
}

func TestCourseListDefaults(t *testing.T) {
	uid := types.NewID()
	mine := sampleCourse(types.NewID(), uid, types.CourseStatusCompleted, false)
	var gotFilter types.CourseListFilter
	svc := newCourseSvc(&mockCourseRepo{
		list: func(userID types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
			require.Equal(t, uid, userID)
			gotFilter = f
			return []types.CourseListRow{{Course: mine, ProgressStatus: types.ProgressStatusInProgress}}, 1, nil
		},
	})
	resp, err := svc.List(context.Background(), uid, nil)
	require.NoError(t, err)
	// nil req → 默认 all / 第 1 页 / 20
	require.Equal(t, types.CourseScopeAll, gotFilter.Scope)
	require.Equal(t, 1, gotFilter.Page)
	require.Equal(t, defaultCoursePageSize, gotFilter.PageSize)
	require.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Items, 1)
	it := resp.Items[0]
	require.True(t, it.Owned)
	require.Equal(t, types.ProgressStatusInProgress, it.Progress)
}

func TestCourseListFilterPassthrough(t *testing.T) {
	uid := types.NewID()
	svc := newCourseSvc(&mockCourseRepo{
		list: func(_ types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
			require.Equal(t, types.CourseScopeMine, f.Scope)
			require.Equal(t, "go", f.Keyword)
			require.Equal(t, types.ProgressStatusUnstarted, f.Progress)
			require.Equal(t, 2, f.Page)
			require.Equal(t, 15, f.PageSize)
			require.Equal(t, 15, f.Offset)
			return nil, 0, nil
		},
	})
	_, err := svc.List(context.Background(), uid, &types.CourseListReq{
		Scope: types.CourseScopeMine, Keyword: "go",
		Progress: types.ProgressStatusUnstarted, Page: 2, PageSize: 15,
	})
	require.NoError(t, err)
}

func TestCourseListInvalidScopeFallsBack(t *testing.T) {
	uid := types.NewID()
	svc := newCourseSvc(&mockCourseRepo{
		list: func(_ types.ID, f types.CourseListFilter) ([]types.CourseListRow, int64, error) {
			require.Equal(t, types.CourseScopeAll, f.Scope)
			return nil, 0, nil
		},
	})
	_, err := svc.List(context.Background(), uid, &types.CourseListReq{Scope: "bogus"})
	require.NoError(t, err)
}

func TestCourseListOwnedFalseForOtherOwner(t *testing.T) {
	uid := types.NewID()
	other := types.NewID()
	pub := sampleCourse(types.NewID(), other, types.CourseStatusCompleted, true)
	svc := newCourseSvc(&mockCourseRepo{
		list: func(_ types.ID, _ types.CourseListFilter) ([]types.CourseListRow, int64, error) {
			return []types.CourseListRow{{Course: pub, ProgressStatus: types.ProgressStatusUnstarted}}, 1, nil
		},
	})
	resp, err := svc.List(context.Background(), uid, &types.CourseListReq{Scope: types.CourseScopePublic})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.False(t, resp.Items[0].Owned, "公共库他人课程 owned 应为 false")
	require.True(t, resp.Items[0].IsPublic)
}

// ---- Create / Outline tests ----

func TestCreateRequiresPromptAndFiles(t *testing.T) {
	uid := types.NewID()
	svc := newCourseSvc(&mockCourseRepo{})

	_, err := svc.Create(context.Background(), uid, &types.CreateCourseReq{Prompt: "  ", Files: []types.UploadedFile{{Name: "a.txt", Data: []byte("x")}}})
	require.ErrorIs(t, err, types.ErrPromptRequired)

	// 无参考文档也允许创建（prompt 文本作为提示词依据）
	_, err = svc.Create(context.Background(), uid, &types.CreateCourseReq{Prompt: "学习数组"})
	require.NoError(t, err)
}

func TestCreateRejectsUnsupportedFile(t *testing.T) {
	uid := types.NewID()
	svc := newCourseSvc(&mockCourseRepo{})
	_, err := svc.Create(context.Background(), uid, &types.CreateCourseReq{
		Prompt: "学习数组",
		Files:  []types.UploadedFile{{Name: "notes.pdf", Data: []byte("%PDF")}},
	})
	require.ErrorIs(t, err, types.ErrUnsupportedFile)
}

func TestCreateStoresCourseAndDocuments(t *testing.T) {
	uid := types.NewID()
	var created *types.Course
	var docCreated *types.Document
	var putKeys []string

	svc := NewCourseService(
		&mockCourseRepo{create: func(c *types.Course) error { created = c; return nil }},
		&mockOutlineRepo{},
		&mockDocRepo{create: func(d *types.Document) error { docCreated = d; return nil }},
		passTM{},
		&mockStorage{put: func(key string, _ io.Reader) (string, error) {
			putKeys = append(putKeys, key)
			return "/uploads/" + key, nil
		}},
		defaultMockSet().cfgSvc,
		model.NewRegistry(),
	)

	resp, err := svc.Create(context.Background(), uid, &types.CreateCourseReq{
		Prompt: "学习数组的声明与访问",
		Files: []types.UploadedFile{
			{Name: "intro.md", Data: []byte("数组是…")},
			{Name: "quiz.txt", Data: []byte("选择题…")},
		},
	})
	require.NoError(t, err)
	require.NotEqual(t, types.NilID, resp.ID)
	require.Equal(t, types.CourseStatusDraft, created.Status)
	require.Equal(t, "学习数组的声明与访问", created.Prompt)
	require.Equal(t, created.ID, resp.ID)
	require.ElementsMatch(t, []string{created.ID.String() + "/intro.md", created.ID.String() + "/quiz.txt"}, putKeys)
	require.NotNil(t, docCreated)
	require.Equal(t, created.ID, docCreated.CourseID)
}

// fakeLLM 固定返回预设内容的 LLM 客户端。
type fakeLLM struct {
	content string
	err     error
}

func (f *fakeLLM) Chat(_ context.Context, _ model.ChatRequest) (*model.ChatResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &model.ChatResponse{Content: f.content}, nil
}
func (f *fakeLLM) ChatStream(context.Context, model.ChatRequest, model.StreamCallback) error {
	return nil
}

func TestGenerateOutlineParsesAndPersists(t *testing.T) {
	uid := types.NewID()
	course := sampleCourse(types.NewID(), uid, types.CourseStatusDraft, false)

	registry := model.NewRegistry()
	registry.RegisterLLM("test", func(model.ProviderConfig) model.LLMClient {
		return &fakeLLM{content: `{"title":"数组课程","sections":[
			{"title":"声明","type":"slide","knowledge_points":["语法"]},
			{"title":"访问","type":"quiz","knowledge_points":["索引"]}
		]}`}
	})

	var persisted types.Outline
	var updatedTitle string
	svc := NewCourseService(
		&mockCourseRepo{getByID: func(_, _ types.ID) (*types.Course, error) { c := course; return &c, nil },
			updateTt: func(_ types.ID, t string) error { updatedTitle = t; return nil }},
		&mockOutlineRepo{create: func(o *types.Outline) error { persisted = *o; return nil }},
		&mockDocRepo{},
		passTM{},
		&mockStorage{},
		&mockModelCfgSvc{resolve: func() (model.ProviderConfig, error) {
			return model.ProviderConfig{Provider: "test", Model: "m", APIKey: "k"}, nil
		}},
		registry,
	)

	res, err := svc.GenerateOutline(context.Background(), uid, course.ID)
	require.NoError(t, err)
	require.Equal(t, "数组课程", res.Title)
	require.Len(t, res.Sections, 2)
	require.Equal(t, "声明", res.Sections[0].Title)
	require.Equal(t, "数组课程", updatedTitle)

	var content types.OutlineContent
	require.NoError(t, json.Unmarshal(persisted.Content, &content))
	require.Len(t, content.Sections, 2)
	require.Equal(t, types.OutlineStatusDraft, persisted.Status)
}

func TestGenerateOutlineRejectsInvalidType(t *testing.T) {
	uid := types.NewID()
	course := sampleCourse(types.NewID(), uid, types.CourseStatusDraft, false)

	registry := model.NewRegistry()
	registry.RegisterLLM("test", func(model.ProviderConfig) model.LLMClient {
		return &fakeLLM{content: `{"title":"x","sections":[{"title":"ok","type":"bogus","knowledge_points":["a"]}]}`}
	})

	svc := NewCourseService(
		&mockCourseRepo{getByID: func(_, _ types.ID) (*types.Course, error) { c := course; return &c, nil }},
		&mockOutlineRepo{},
		&mockDocRepo{},
		passTM{},
		&mockStorage{},
		&mockModelCfgSvc{resolve: func() (model.ProviderConfig, error) {
			return model.ProviderConfig{Provider: "test", Model: "m", APIKey: "k"}, nil
		}},
		registry,
	)

	_, err := svc.GenerateOutline(context.Background(), uid, course.ID)
	require.ErrorIs(t, err, types.ErrOutlineFailed)
}
