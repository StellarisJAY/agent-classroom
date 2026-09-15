package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// ---------- 讨论模式（discussion.go）单测 ----------

// recordingConvRepo 内存记录 Append 的序列，供 ListMessages 回放。
type recordingConvRepo struct {
	mu    sync.Mutex
	convs map[types.ID][]types.Message
}

var _ types.ConversationRepo = (*recordingConvRepo)(nil)

func (r *recordingConvRepo) GetOrCreate(_ context.Context, courseID, userID types.ID) (*types.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	conv := &types.Conversation{ID: types.NewID(), CourseID: courseID, UserID: userID}
	r.convs[conv.ID] = nil
	return conv, nil
}

func (r *recordingConvRepo) ListMessages(_ context.Context, conversationID types.ID) ([]types.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]types.Message, len(r.convs[conversationID]))
	copy(out, r.convs[conversationID])
	return out, nil
}

func (r *recordingConvRepo) Append(_ context.Context, conversationID types.ID, role string, content json.RawMessage, sectionID *types.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.convs[conversationID] = append(r.convs[conversationID], types.Message{
		ID:             types.NewID(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		SectionID:      sectionID,
	})
	return nil
}

func newRecordingConvRepo() *recordingConvRepo {
	return &recordingConvRepo{convs: make(map[types.ID][]types.Message)}
}

// recordingSink 记录 SSE 事件序列的 types.DiscussionSink 实现。
type recordingSink struct {
	mu      sync.Mutex
	texts   []string
	actions []string
	ended   bool
	errs    []string
}

func (s *recordingSink) Text(delta string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.texts = append(s.texts, delta)
	return nil
}

func (s *recordingSink) Action(name string, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions = append(s.actions, name)
	return nil
}

func (s *recordingSink) End() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ended = true
	return nil
}

func (s *recordingSink) Error(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, msg)
}

// scriptLLM 按脚本逐轮返回（tool_call 轮 / 文本轮），实现 model.ToolStreamClient。
type scriptLLM struct {
	mu    sync.Mutex
	turns []func(model.StreamHandler) error
}

func (f *scriptLLM) Chat(context.Context, model.ChatRequest) (*model.ChatResponse, error) {
	return &model.ChatResponse{}, nil
}
func (f *scriptLLM) ChatStream(_ context.Context, _ model.ChatRequest, cb model.StreamCallback) error {
	return nil
}
func (f *scriptLLM) ChatToolStream(_ context.Context, _ model.ChatRequest, h model.StreamHandler) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.turns) == 0 {
		return nil
	}
	fn := f.turns[0]
	f.turns = f.turns[1:]
	return fn(h)
}

func scriptedRegistry(next func() func(model.StreamHandler) error) *model.Registry {
	reg := model.NewRegistry()
	reg.SetDefaultLLMFactory(func(model.ProviderConfig) model.LLMClient {
		return &scriptLLM{turns: []func(model.StreamHandler) error{next()}}
	})
	return reg
}

// newDiscussionFixture 构建讨论服务（含一个 completed 课程与可定制环节列表）。
func newDiscussionFixture(course *types.Course, secs []types.Section, registry *model.Registry, convRepo types.ConversationRepo, qRepo types.QuestionRepo) *DiscussionService {
	if registry == nil {
		registry = model.NewRegistry()
		registry.SetDefaultLLMFactory(func(model.ProviderConfig) model.LLMClient { return &scriptLLM{} })
	}
	if convRepo == nil {
		convRepo = newRecordingConvRepo()
	}
	if qRepo == nil {
		qRepo = &mockQuestionRepo{}
	}
	return &DiscussionService{
		courseRepo: &mockCourseRepo{getByID: func(_ types.ID, id types.ID) (*types.Course, error) {
			if id == course.ID {
				return course, nil
			}
			return nil, types.ErrNotFound
		}},
		sectionRepo:  &mockSectionRepo{listBy: func(types.ID) ([]types.Section, error) { return secs, nil }},
		questionRepo: qRepo,
		convRepo:     convRepo,
		modelCfgSvc: &mockModelCfgSvc{resolve: func() (model.ProviderConfig, error) {
			return model.ProviderConfig{Provider: "openai", Model: "gpt-4o-mini", APIKey: "k", BaseURL: "http://x"}, nil
		}},
		registry: registry,
		running:  make(map[types.ID]struct{}),
	}
}

// ---------- 业务校验 ----------

func TestDiscussionAskRejectsGeneratingSection(t *testing.T) {
	courseID := types.NewID()
	course := &types.Course{ID: courseID, OwnerID: types.NewID(), Title: "t", Status: types.CourseStatusCompleted, Thinking: "default"}
	secs := []types.Section{
		{ID: types.NewID(), CourseID: courseID, Position: 1, Type: types.SectionTypeSlide, Status: types.SectionStatusDone},
		{ID: types.NewID(), CourseID: courseID, Position: 2, Type: types.SectionTypeQuiz, Status: types.SectionStatusGenerating},
	}
	svc := newDiscussionFixture(course, secs, nil, nil, nil)

	err := svc.Ask(context.Background(), course.OwnerID, courseID, &types.AskQuestionReq{Question: "你好"}, &recordingSink{})
	var be *types.BizError
	require.True(t, errors.As(err, &be))
	require.Equal(t, types.ErrDiscussionGenerating.Code, be.Code)
}

func TestDiscussionAskConcurrentBusy(t *testing.T) {
	courseID := types.NewID()
	course := &types.Course{ID: courseID, OwnerID: types.NewID(), Title: "t", Status: types.CourseStatusCompleted, Thinking: "default"}
	svc := newDiscussionFixture(course, nil, nil, nil, nil)
	require.True(t, svc.tryLock(courseID))
	require.False(t, svc.tryLock(courseID))
	svc.unlock(courseID)
	require.True(t, svc.tryLock(courseID))
}

// ---------- 正常路径：事件顺序与落库时序 ----------

func TestDiscussionAskEventsAndPersistence(t *testing.T) {
	courseID := types.NewID()
	sectionID := types.NewID()
	course := &types.Course{ID: courseID, OwnerID: types.NewID(), Title: "初识数组", Status: types.CourseStatusCompleted, Thinking: "default"}
	secs := []types.Section{{
		ID: sectionID, CourseID: courseID, Position: 1,
		Type: types.SectionTypeSlide, Title: "初识数组", Status: types.SectionStatusDone,
		Content: datatypes.JSON(`{"elements":[{"id":"e1","type":"text","content":"数组是集合"}]}`),
		Steps:   datatypes.JSON(`[{"text":"先看概念","actions":[{"type":"highlight","targetElementId":"e1"}]}]`),
	}}

	// 脚本：第 1 轮文本增量 + 工具调用；第 2 轮纯文本收尾。
	callID := "call-1"
	reg := model.NewRegistry()
	reg.SetDefaultLLMFactory(func(model.ProviderConfig) model.LLMClient {
		return &scriptLLM{turns: []func(model.StreamHandler) error{
			func(h model.StreamHandler) error {
				if h.OnText != nil {
					_ = h.OnText("先切换")
				}
				if h.OnToolCall != nil {
					return h.OnToolCall([]model.ToolCall{{
						ID: callID, Function: model.ToolCallFunc{
							Name:      "jump_to_section",
							Arguments: `{"section_id":"` + sectionID.String() + `"}`,
						}}})
				}
				return nil
			},
			func(h model.StreamHandler) error {
				if h.OnText != nil {
					_ = h.OnText(" monocolor结束")
				}
				return nil
			},
		}}
	})

	convRepo := newRecordingConvRepo()
	svc := newDiscussionFixture(course, secs, reg, convRepo, nil)

	sink := &recordingSink{}
	section := sectionID
	err := svc.Ask(context.Background(), course.OwnerID, courseID,
		&types.AskQuestionReq{Question: "帮我讲一下数组", SectionID: &section, StepIndex: intPtr(0)}, sink)
	require.NoError(t, err)
	require.True(t, sink.ended)
	require.Empty(t, sink.errs)
	require.Equal(t, []string{"jump_to_section"}, sink.actions)

	// 落库序列：user → assistant(tool_calls) → tool → assistant(text)
	convOut, err := convRepo.ListMessages(context.Background(), convRepoPendingID(t, convRepo))
	require.NoError(t, err)
	require.Len(t, convOut, 4)
	require.Equal(t, types.MessageRoleUser, convOut[0].Role)
	require.Equal(t, section.String(), convOut[0].SectionID.String())
	require.Equal(t, types.MessageRoleAssistant, convOut[1].Role)
	var callContent types.MessageContent
	require.NoError(t, json.Unmarshal(convOut[1].Content, &callContent))
	require.Len(t, callContent.ToolCalls, 1)
	require.Equal(t, callID, callContent.ToolCalls[0].ID)
	require.Equal(t, "jump_to_section", callContent.ToolCalls[0].Name)
	require.Equal(t, types.MessageRoleTool, convOut[2].Role)
	// tool 消息不携带 section_id
	require.Nil(t, convOut[1].SectionID)
	require.Nil(t, convOut[2].SectionID)
	require.Equal(t, types.MessageRoleAssistant, convOut[3].Role)
	var finalText types.MessageContent
	require.NoError(t, json.Unmarshal(convOut[3].Content, &finalText))
	require.Equal(t, " monocolor结束", finalText.Text)
}

// ---------- 上下文装配：quiz 剔除答案 ----------

func TestDiscussionQuizAnswerStripped(t *testing.T) {
	courseID := types.NewID()
	course := types.Course{ID: courseID, Title: "测试", Prompt: "主题", Thinking: "default"}
	sec := types.Section{
		ID: types.NewID(), CourseID: courseID, Type: types.SectionTypeQuiz,
		Title: "测验", Status: types.SectionStatusDone,
	}
	qRepo := &mockQuestionRepo{listBy: func(types.ID) ([]types.Question, error) {
		opts, _ := json.Marshal([]string{"A 选项", "B 选项"})
		ans, _ := json.Marshal([]int{1})
		expl, _ := json.Marshal([]string{"B 是正确答案", "A 错误"})
		return []types.Question{{
			ID: types.NewID(), SectionID: sec.ID, Position: 1,
			Type: types.QuestionTypeSingle, Stem: "一维数组的长度必须固定吗？",
			Options: opts, Answers: ans, Explanations: expl,
		}}, nil
	}}

	section := sec
	svcQuiz := newDiscussionFixture(&course, []types.Section{sec}, nil, nil, qRepo)
	system, err := svcQuiz.buildSystem(context.Background(), &course, []types.Section{section},
		&types.AskQuestionReq{Question: "答案是什么", SectionID: &section.ID})
	require.NoError(t, err)
	require.Contains(t, system, "一维数组的长度必须固定吗？")
	// 装配层硬剔除：答案与解释绝不进入提示词
	require.NotContains(t, system, "B 是正确答案")
	require.NotContains(t, system, "A 错误")
}

func convRepoPendingID(_ *testing.T, r *recordingConvRepo) types.ID {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id := range r.convs {
		return id
	}
	return types.NilID
}

// ---------- lruWindow ----------

func TestLRUWindowPairTrim(t *testing.T) {
	// 3 轮对话，user(小) → assistant(tool_call, 超大) → tool；窗口限制卡在中间轮，
	// 截断起点必须回退到 user 边界，保持 assistant(tool_call) ↔ tool 成对。
	var msgs []model.ChatMessage
	add := func(role model.Role, content string, calls []model.ToolCall, toolCallID string) {
		msgs = append(msgs, model.ChatMessage{Role: role, Content: content, ToolCalls: calls, ToolCallID: toolCallID})
	}
	for i := 0; i < 3; i++ {
		add(model.RoleUser, fmt.Sprintf("第%d问", i), nil, "")
		add(model.RoleAssistant, "", []model.ToolCall{{ID: "c" + strconv.Itoa(i), Function: model.ToolCallFunc{Name: "highlight", Arguments: strings.Repeat("y", 60)}}}, "")
		add(model.RoleTool, strings.Repeat("x", 40), nil, "")
	}
	out := lruWindow(msgs, 250)
	require.NotEmpty(t, out)
	// 成对还原：tool 消息前面必须是 assistant(tool_calls)
	for i, m := range out {
		if m.Role == model.RoleTool {
			prev := out[i-1]
			require.Equal(t, model.RoleAssistant, prev.Role)
			require.Len(t, prev.ToolCalls, 1)
		}
	}
	// 截断起点必须是 user 消息（或整段保留）
	require.Equal(t, model.RoleUser, out[0].Role)
}

func intPtr(i int) *int { return &i }
