package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/StellarisJAY/agent-classroom/internal/agent"
	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/StellarisJAY/agent-classroom/internal/types"
)

// 讨论模式业务实现：服务端装配上下文 → agent loop（internal/agent）→
// 事件经 types.DiscussionSink 流出 → 消息逐条落库。
// 设计核对 docs/讨论模式方案.md：单流 + 合成工具结果，前端为动作执行器。

const (
	// historyWindowCharBudget 讨论历史近似 token 窗口（约 16k tokens），超限丢最旧。
	historyWindowCharBudget = 48000
	// discussionMaxTurns agent loop 轮次上限（含首次提问轮）。
	discussionMaxTurns = 8
)

// DiscussionService 课程级问答会话业务实现。
type DiscussionService struct {
	courseRepo   types.CourseRepo
	sectionRepo  types.SectionRepo
	questionRepo types.QuestionRepo
	convRepo     types.ConversationRepo
	modelCfgSvc  types.ModelConfigService
	registry     *model.Registry

	// running 进行中会话（per-conversation 串行）：同会话进行中提问直接拒绝。
	mu      sync.Mutex
	running map[types.ID]struct{}
}

var _ types.DiscussionService = (*DiscussionService)(nil)

// NewDiscussionService 创建讨论模式业务实现。
func NewDiscussionService(
	courseRepo types.CourseRepo,
	sectionRepo types.SectionRepo,
	questionRepo types.QuestionRepo,
	convRepo types.ConversationRepo,
	modelCfgSvc types.ModelConfigService,
	registry *model.Registry,
) types.DiscussionService {
	return &DiscussionService{
		courseRepo:   courseRepo,
		sectionRepo:  sectionRepo,
		questionRepo: questionRepo,
		convRepo:     convRepo,
		modelCfgSvc:  modelCfgSvc,
		registry:     registry,
		running:      make(map[types.ID]struct{}),
	}
}

// resolveConversation 解析本次提问的目标会话：
// req.ConversationID 非空则取该会话（校验属主），否则隐式新建一条会话（新对话）。
func (s *DiscussionService) resolveConversation(ctx context.Context, userID, courseID types.ID, req types.AskQuestionReq) (*types.Conversation, error) {
	if req.ConversationID != nil && *req.ConversationID != types.NilID {
		conv, err := s.convRepo.GetByID(ctx, userID, *req.ConversationID)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				return nil, types.ErrConversationNotFound
			}
			return nil, err
		}
		return conv, nil
	}
	conv, err := s.convRepo.Create(ctx, courseID, userID)
	if err != nil {
		return nil, err
	}
	// 新建的会话对象还未写 title；首问在 Ask 内统一写入。
	return conv, nil
}

// conversationTitle 取提问前 10 个字符作为会话标题。
func conversationTitle(question string) string {
	q := strings.TrimSpace(question)
	if r := []rune(q); len(r) > 10 {
		return string(r[:10])
	}
	return q
}

// Ask 处理一次提问。整体流程：
//
//	课程校验（owner + 全部环节 done）→ 解析会话（指定或隐式新建）→ 落库 user 消息
//	（首问时以提问前 10 字写入会话标题）→ 装配 system（人设 + 课程 上下文 + 历史窗口）→
//	agent.Runner.Run（工具合成 success）→ OnText/OnToolCall → sink，OnMessage → 逐条落库。
func (s *DiscussionService) Ask(ctx context.Context, userID, courseID types.ID, req types.AskQuestionReq, sink types.DiscussionSink) error {
	course, err := s.courseRepo.GetByID(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return types.ErrCourseNotFound
		}
		return err
	}
	secs, err := s.sectionRepo.ListByCourse(ctx, course.ID)
	if err != nil {
		return err
	}
	// 生成中禁问（前端置灰 + 后端校验）：课程存在任一未完成环节即拒绝。
	for _, sec := range secs {
		if sec.Status != types.SectionStatusDone {
			return types.ErrDiscussionGenerating
		}
	}

	// 解析会话在锁之前：锁按会话粒度，不同会话可并行提问。
	conv, err := s.resolveConversation(ctx, userID, course.ID, req)
	if err != nil {
		return err
	}

	discussionClient, thinking, err := s.resolveClient(ctx, userID, course)
	if err != nil {
		return err
	}

	// per-conversation 并发锁。
	if !s.tryLock(conv.ID) {
		return types.ErrDiscussionBusy
	}
	defer s.unlock(conv.ID)

	// 提问先落库（带来源环节），随后的 loop 消息逐条追加。
	if err := s.appendMessage(ctx, conv.ID, req.SectionID, model.ChatMessage{Role: model.RoleUser, Content: req.Question}); err != nil {
		return err
	}
	// 会话首次提问：以提问前 10 个字符作为标题（repo 层仅在 title 为空时写入）。
	_ = s.convRepo.UpdateTitle(ctx, conv.ID, conversationTitle(req.Question))
	// 构建agent上下文
	messages, err := s.buildMessages(ctx, course, secs, req, conv.ID)
	if err != nil {
		return err
	}
	// agent loop开始执行，回答问题+动作执行
	runner := &agent.Runner{Client: discussionClient, MaxTurns: discussionMaxTurns}
	err = runner.Run(ctx, agent.Request{
		Messages: messages,
		Tools:    buildDiscussionTools(currentSectionType(secs, req.SectionID)),
		Thinking: thinking,
		Handler: agent.Handler{
			// agent输出文本
			OnText: func(_ context.Context, delta string) error {
				return sink.Text(delta)
			},
			// agent工具调用
			OnToolCall: func(_ context.Context, calls []model.ToolCall) error {
				// 因为讨论模式智能体的工具动作只作用于前端页面，没有持久性，所以只要发送sse成功就视为工具调用成功
				for _, call := range calls {
					if aerr := sink.Action(call.Function.Name, call.Function.Arguments); aerr != nil {
						return aerr
					}
				}
				return nil
			},
			// agent消息落库
			OnMessage: func(_ context.Context, msg model.ChatMessage) error {
				return s.appendMessage(ctx, conv.ID, req.SectionID, msg)
			},
		},
	})
	if err != nil {
		sink.Error("回答过程出现异常，已中断；请重新提问")
		slog.Error("discussion loop error", "error", err)
		return fmt.Errorf("discussion loop: %w", err)
	}
	return sink.End()
}

// ListConversations 返回某课程下当前用户全部会话（按最近活跃倒序）。
func (s *DiscussionService) ListConversations(ctx context.Context, userID, courseID types.ID) ([]types.ConversationItemResp, error) {
	if _, err := s.courseRepo.GetByID(ctx, userID, courseID); err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}
	convs, err := s.convRepo.ListByCourse(ctx, courseID, userID)
	if err != nil {
		return nil, err
	}
	out := make([]types.ConversationItemResp, 0, len(convs))
	for _, c := range convs {
		out = append(out, types.ConversationItemResp{
			ID:       c.ID,
			Title:    c.Title,
			UpdateAt: c.UpdateAt,
		})
	}
	return out, nil
}

// ListConversation 返回指定会话的问答历史（仅 owner）；conversationID 为空时
// 取最近活跃会话，课程尚无会话返回空数组（不隐式建会话）。
func (s *DiscussionService) ListConversation(ctx context.Context, userID, courseID, conversationID types.ID) ([]types.ConversationMessageResp, error) {
	if _, err := s.courseRepo.GetByID(ctx, userID, courseID); err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrCourseNotFound
		}
		return nil, err
	}
	convID := conversationID
	if convID == types.NilID {
		convs, err := s.convRepo.ListByCourse(ctx, courseID, userID)
		if err != nil {
			return nil, err
		}
		if len(convs) == 0 {
			return []types.ConversationMessageResp{}, nil
		}
		convID = convs[0].ID
	} else if _, err := s.convRepo.GetByID(ctx, userID, convID); err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return nil, types.ErrConversationNotFound
		}
		return nil, err
	}
	msgs, err := s.convRepo.ListMessages(ctx, convID)
	if err != nil {
		return nil, err
	}
	out := make([]types.ConversationMessageResp, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, types.ConversationMessageResp{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			SectionID: m.SectionID,
			CreatedAt: m.CreateAt,
		})
	}
	return out, nil
}

// ---- 内部装配 ----

// resolveClient 解析讨论用模型客户端：课程绑定配置优先，否则用户默认配置。
// 返回 ToolStreamClient（OpenAI 兼容实现支持）与思考限制。
func (s *DiscussionService) resolveClient(ctx context.Context, userID types.ID, course *types.Course) (model.ToolStreamClient, string, error) {
	var cfg model.ProviderConfig
	var err error
	if course.ModelConfigID != nil && *course.ModelConfigID != types.NilID {
		cfg, err = s.modelCfgSvc.ResolveByID(ctx, userID, *course.ModelConfigID)
	} else {
		cfg, err = s.modelCfgSvc.ResolveDefault(ctx, userID)
	}
	if err != nil {
		return nil, "", err
	}
	if cfg.Model == "" || cfg.APIKey == "" {
		return nil, "", types.ErrNoModelConfig
	}
	client := s.registry.NewLLM(cfg)
	if client == nil {
		return nil, "", types.ErrNoModelConfig
	}
	stream, ok := client.(model.ToolStreamClient)
	if !ok {
		return nil, "", types.ErrNoModelConfig
	}
	return stream, course.Thinking, nil
}

// buildMessages 装配本次提问的初始消息序列：
// system（人设 + 课程/环节上下文）+ 历史窗口（LRU 截断）+ 本轮 user 消息。
func (s *DiscussionService) buildMessages(ctx context.Context, course *types.Course, secs []types.Section, req types.AskQuestionReq, convID types.ID) ([]model.ChatMessage, error) {
	system, err := s.buildSystem(ctx, course, secs, req)
	if err != nil {
		return nil, err
	}

	dbMsgs, err := s.convRepo.ListMessages(ctx, convID)
	if err != nil {
		return nil, err
	}
	history := make([]model.ChatMessage, 0, len(dbMsgs))
	for _, m := range dbMsgs {
		cm, cerr := dbToChat(m)
		if cerr != nil {
			// 单条消息损坏不阻断对话，跳过并记日志。
			continue
		}
		history = append(history, cm)
	}
	history = lruWindow(history, historyWindowCharBudget)

	return append([]model.ChatMessage{{Role: model.RoleSystem, Content: system}}, history...), nil
}

// currentSectionType 返回提问来源环节的类型；无来源（全局提问）时第一环节？
// 无来源按 slide 全量工具处理最安全（返回空串 → buildDiscussionTools 全量）。
func currentSectionType(secs []types.Section, sectionID *types.ID) string {
	if sectionID == nil {
		return ""
	}
	for _, sec := range secs {
		if sec.ID == *sectionID {
			return sec.Type
		}
	}
	return ""
}

// buildSystem 组装讨论模式 system 提示词：
// 人设/工具规则（embed 的 prompts/discussion.md）+ 课程上下文 + 环节大纲归约 +
// 当前环节标注（含步骤索引）。quiz 环节在此硬剔除答案与解释。
func (s *DiscussionService) buildSystem(ctx context.Context, course *types.Course, secs []types.Section, req types.AskQuestionReq) (string, error) {
	var b strings.Builder
	b.WriteString(discussionSystemPrompt)
	b.WriteString("\n\n# [course]\n\n")
	b.WriteString("标题: ")
	b.WriteString(course.Title)
	b.WriteString("\n")
	b.WriteString("主题: ")
	b.WriteString(course.Prompt)
	b.WriteString("\n")

	b.WriteString("\n# [sections]\n\n")
	for i := range secs {
		sec := &secs[i]
		kp := strings.Join(unmarshalKP(sec.KnowledgePoints), "、")
		fmt.Fprintf(&b, "- section_id: %s | 类型: %s | 标题: %s | 知识点: %s\n",
			sec.ID, sec.Type, sec.Title, kp)
		detail, err := s.summarizeSection(ctx, sec)
		if err != nil {
			return "", err
		}
		b.WriteString(detail)
	}

	b.WriteString("\n\n# [current_section]\n\n")
	cur := findSectionByID(secs, req.SectionID)
	if cur == nil {
		b.WriteString("学生此刻未处于任何环节（全局提问）。动作类工具将作用于上一处讲解位置；跨环节请先 jump_to_section。\n")
	} else {
		kp := strings.Join(unmarshalKP(cur.KnowledgePoints), "、")
		fmt.Fprintf(&b, "section_id: %s | 类型: %s | 标题: %s | 知识点: %s\n",
			cur.ID, cur.Type, cur.Title, kp)
		detail, err := s.currentSectionDetail(ctx, cur, req.StepIndex)
		if err != nil {
			return "", err
		}
		b.WriteString(detail)
	}
	return b.String(), nil
}

// findSectionByID 按 ID 查找环节。
func findSectionByID(secs []types.Section, id *types.ID) *types.Section {
	if id == nil {
		return nil
	}
	for i := range secs {
		if secs[i].ID == *id {
			return &secs[i]
		}
	}
	return nil
}

// summarizeSection 将环节产物归约为纯文本大纲（每元素/每步骤一行）。
// quiz 只列题面与选项，硬剔除 answers/explanations（防作弊主防线，装配层保证）。
func (s *DiscussionService) summarizeSection(ctx context.Context, sec *types.Section) (string, error) {
	var b strings.Builder
	switch sec.Type {
	case types.SectionTypeSlide:
		var content types.SlideContent
		if len(sec.Content) > 0 {
			if err := json.Unmarshal(sec.Content, &content); err != nil {
				return "", fmt.Errorf("unmarshal slide content %s: %w", sec.ID, err)
			}
		}
		for _, el := range content.Elements {
			fmt.Fprintf(&b, "  元素 %s (%s): %s\n", el.ID, el.Type, slideElementText(el))
		}
		var steps []types.SlideStep
		if len(sec.Steps) > 0 {
			if err := json.Unmarshal(sec.Steps, &steps); err != nil {
				return "", fmt.Errorf("unmarshal slide steps %s: %w", sec.ID, err)
			}
		}
		for i, step := range steps {
			acts := make([]string, 0, len(step.Actions))
			for _, a := range step.Actions {
				if a.TargetElementID != "" {
					acts = append(acts, a.Type+": "+a.TargetElementID)
					continue
				}
				acts = append(acts, a.Type)
			}
			if len(acts) == 0 {
				fmt.Fprintf(&b, "  步骤 %d: %s\n", i, step.Text)
				continue
			}
			fmt.Fprintf(&b, "  步骤 %d: %s (动作: %s)\n", i, step.Text, strings.Join(acts, ", "))
		}
	case types.SectionTypeQuiz:
		qs, err := s.questionRepo.ListBySection(ctx, sec.ID)
		if err != nil {
			return "", err
		}
		for i, q := range qs {
			fmt.Fprintf(&b, "  题 %d (%s): %s\n", i+1, q.Type, q.Stem)
			for j, opt := range unmarshalStringSlice(q.Options) {
				fmt.Fprintf(&b, "    选项 [%d]: %s\n", j, opt)
			}
		}
		b.WriteString("  （提示：仅供做过的题目维护上下文；你不掌握本题答案）\n")
	case types.SectionTypeDemo3D, types.SectionTypeDemoFunction, types.SectionTypeDemoBasic:
		b.WriteString("  （互动演示环节：仅可 jump_to_section 切入，动作类工具不可用）\n")
	default:
		b.WriteString("\n")
	}
	return b.String(), nil
}

// currentSectionDetail 汇报学生当前所处环节与步骤位置（详细版，含元素/步骤清单）。
func (s *DiscussionService) currentSectionDetail(ctx context.Context, sec *types.Section, stepIndex *int) (string, error) {
	detail, err := s.summarizeSection(ctx, sec)
	if err != nil {
		return "", err
	}
	pos := "末步之后"
	if stepIndex != nil {
		pos = fmt.Sprintf("step_index %d", *stepIndex)
	}
	return fmt.Sprintf("学生当前步骤: %s\n详细产物:\n%s", pos, detail), nil
}

// slideElementText 将元素归约为一行文本描述（供模型理解元素内容）。
func slideElementText(el types.SlideElement) string {
	switch el.Type {
	case types.SlideElementList:
		return strings.Join(el.Items, " / ")
	case types.SlideElementShape:
		if el.Label != "" {
			return el.Label
		}
		return el.Shape
	case types.SlideElementImage:
		if el.Src != "" {
			return "[图片] " + el.Prompt
		}
		return "[图片占位] " + el.Prompt
	case types.SlideElementFormula, types.SlideElementText, types.SlideElementMermaid:
		if len(el.Content) > 200 {
			return el.Content[:200]
		}
		return el.Content
	case types.SlideElementChart:
		return fmt.Sprintf("图表 %s: %s (类目: %s)", el.Chart, el.Title, strings.Join(el.Categories, "、"))
	default:
		if el.Content != "" {
			return el.Content
		}
		return ""
	}
}

// lruWindow 近似 token 窗口的 LRU 截断：从新到旧装载，累计字符数超预算丢最旧。
// 截断起点回退到最近一条 user 消息，保证 assistant(tool_call) ↔ tool 成对完整。
// system 恒定组装，不参与截断。
func lruWindow(msgs []model.ChatMessage, budget int) []model.ChatMessage {
	if msgs == nil {
		msgs = []model.ChatMessage{}
	}
	total := 0
	for _, m := range msgs {
		total += chatMessageSize(m)
	}
	if total <= budget {
		return cloneMessages(msgs)
	}
	// 从新到旧累计；超限后回退到最近的 user 边界。
	acc := 0
	overflowIdx := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		acc += chatMessageSize(msgs[i])
		if acc > budget {
			overflowIdx = i
			break
		}
	}
	start := len(msgs)
	if overflowIdx >= 0 {
		for i := overflowIdx; i < len(msgs); i++ {
			if msgs[i].Role == model.RoleUser {
				start = i
				break
			}
		}
	}
	if start >= len(msgs) {
		// 尾部没有 user 边界可截（理论罕见）：原样返回全量，宁可超预算也不破坏成对序列。
		return cloneMessages(msgs)
	}
	return cloneMessages(msgs[start:])
}

// cloneMessages 复制一份消息切片，避免直接引用底层数组（供后续追加时不干扰原数据）。
func cloneMessages(msgs []model.ChatMessage) []model.ChatMessage {
	out := make([]model.ChatMessage, len(msgs))
	copy(out, msgs)
	return out
}

// chatMessageSize 近似估算一条消息的字符数（正文 + 工具参数）。
func chatMessageSize(m model.ChatMessage) int {
	n := len(m.Content)
	for _, tc := range m.ToolCalls {
		n += len(tc.Function.Arguments) + len(tc.Function.Name) + len(tc.ID)
	}
	return n
}

// dbToChat 将落库的 message 转回 model.ChatMessage（严格还原 assistant(tool_call) ↔ tool 成对）。
func dbToChat(m types.Message) (model.ChatMessage, error) {
	var mc types.MessageContent
	if err := json.Unmarshal(m.Content, &mc); err != nil {
		return model.ChatMessage{}, fmt.Errorf("unmarshal message content %s: %w", m.ID, err)
	}
	switch m.Role {
	case types.MessageRoleUser:
		return model.ChatMessage{Role: model.RoleUser, Content: mc.Text}, nil
	case types.MessageRoleAssistant:
		if len(mc.ToolCalls) > 0 {
			calls := make([]model.ToolCall, 0, len(mc.ToolCalls))
			for _, tc := range mc.ToolCalls {
				calls = append(calls, model.ToolCall{
					ID:   tc.ID,
					Type: model.ToolCallTypeFunction,
					Function: model.ToolCallFunc{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				})
			}
			return model.ChatMessage{Role: model.RoleAssistant, ToolCalls: calls}, nil
		}
		return model.ChatMessage{Role: model.RoleAssistant, Content: mc.Text}, nil
	case types.MessageRoleTool:
		return model.ChatMessage{
			Role:       model.RoleTool,
			ToolCallID: mc.ToolCallID,
			Name:       mc.Name,
			Content:    mc.Result,
		}, nil
	default:
		return model.ChatMessage{}, fmt.Errorf("unknown message role %q (%s)", m.Role, m.ID)
	}
}

// appendMessage 把 loop 产生的消息按方案 §6 的 jsonb 结构逐条落库。
func (s *DiscussionService) appendMessage(ctx context.Context, convID types.ID, sectionID *types.ID, msg model.ChatMessage) error {
	var mc types.MessageContent
	switch msg.Role {
	case model.RoleAssistant:
		if len(msg.ToolCalls) > 0 {
			calls := make([]types.ToolCallEx, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				calls = append(calls, types.ToolCallEx{
					ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
				})
			}
			mc.ToolCalls = calls
		} else {
			mc.Text = msg.Content
		}
	case model.RoleTool:
		mc.ToolCallID = msg.ToolCallID
		mc.Name = msg.Name
		mc.Result = msg.Content
	case model.RoleUser:
		mc.Text = msg.Content
	default:
		return fmt.Errorf("unexpected runtime role %q", msg.Role)
	}
	raw, err := json.Marshal(mc)
	if err != nil {
		return err
	}
	// 只有 user 消息携带 section_id（提问来源）；assistant/tool 一律不携带。
	sid := sectionID
	if msg.Role != model.RoleUser {
		sid = nil
	}
	if err := s.convRepo.Append(ctx, convID, string(msg.Role), raw, sid); err != nil {
		return err
	}
	return nil
}

// tryLock 抢占课程会话的处理权（已有提问进行中返回 false）。
func (s *DiscussionService) tryLock(courseID types.ID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[courseID]; ok {
		return false
	}
	s.running[courseID] = struct{}{}
	return true
}

// unlock 释放课程会话处理锁。
func (s *DiscussionService) unlock(courseID types.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, courseID)
}
