package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/StellarisJAY/agent-classroom/internal/model"
	"github.com/stretchr/testify/require"
)

// scriptTurn 预设一轮 LLM 输出：文本分片 + 工具调用。
type scriptTurn struct {
	deltas []string
	calls  []model.ToolCall
}

// scriptClient 脚本化假客户端：按序播放各轮输出，记录收到的请求。
type scriptClient struct {
	turns []scriptTurn
	got   []model.ChatRequest
	i     int
}

func (f *scriptClient) ChatToolStream(ctx context.Context, req model.ChatRequest, h model.StreamHandler) error {
	f.got = append(f.got, req)
	if f.i >= len(f.turns) {
		return errors.New("脚本轮次耗尽")
	}
	t := f.turns[f.i]
	f.i++
	for _, d := range t.deltas {
		if h.OnText != nil {
			if err := h.OnText(d); err != nil {
				return err
			}
		}
	}
	if len(t.calls) > 0 && h.OnToolCall != nil {
		if err := h.OnToolCall(t.calls); err != nil {
			return err
		}
	}
	return nil
}

func newRunner(c model.ToolStreamClient, maxTurns int) *Runner {
	return &Runner{Client: c, MaxTurns: maxTurns}
}

// side 记录全部回调副作用。
type side struct {
	seq    []string
	texts  string
	called [][]model.ToolCall
}

// collect 装配一个记录全部回调的 Handler 与副作用收集器。
func collect() (Handler, *side) {
	s := &side{}
	return Handler{
		OnText: func(ctx context.Context, delta string) error {
			s.texts += delta
			return nil
		},
		OnToolCall: func(ctx context.Context, calls []model.ToolCall) error {
			s.called = append(s.called, calls)
			return nil
		},
		OnMessage: func(ctx context.Context, msg model.ChatMessage) error {
			switch {
			case msg.Role == model.RoleAssistant && len(msg.ToolCalls) > 0:
				s.seq = append(s.seq, "assistant(tool_calls)")
			case msg.Role == model.RoleAssistant:
				s.seq = append(s.seq, "assistant(text:"+msg.Content+")")
			case msg.Role == model.RoleTool:
				s.seq = append(s.seq, "tool("+msg.Name+":"+msg.Content+")")
			}
			return nil
		},
	}, s
}

// 纯文本收尾：不触发 OnToolCall / tool 消息，初始消息原样透传首轮请求。
func TestRunTextFinish(t *testing.T) {
	c := &scriptClient{turns: []scriptTurn{{deltas: []string{"你好", "呀"}}}}
	h, s := collect()
	err := newRunner(c, 0).Run(context.Background(), Request{
		Messages: []model.ChatMessage{{Role: model.RoleUser, Content: "hi"}},
		Tools: []Tool{{
			Definition: model.Tool{Type: "function", Function: model.ToolFunction{Name: "noop"}},
			Execute:    func(context.Context, string) (string, error) { return "", nil },
		}},
		Handler: h,
	})
	require.NoError(t, err)
	require.Equal(t, "你好呀", s.texts)
	require.Empty(t, s.called)
	require.Equal(t, []string{"assistant(text:你好呀)"}, s.seq)

	require.Len(t, c.got, 1)
	require.Len(t, c.got[0].Messages, 1)
	require.Equal(t, model.RoleUser, c.got[0].Messages[0].Role)
}

// 单轮多工具：assistant(tool_calls) → OnToolCall → 逐条 tool 结果，
// 成对回传下一轮；工具执行失败以错误文本回传、不中断 loop。
func TestRunToolLoopAndNextRound(t *testing.T) {
	c := &scriptClient{turns: []scriptTurn{
		{calls: []model.ToolCall{
			{ID: "c1", Function: model.ToolCallFunc{Name: "highlight", Arguments: `{"id":1}`}},
			{ID: "c2", Function: model.ToolCallFunc{Name: "jump", Arguments: `{}`}},
		}},
		{deltas: []string{"讲完了"}},
	}}
	h, s := collect()
	err := newRunner(c, 0).Run(context.Background(), Request{
		Tools: []Tool{
			{
				Definition: model.Tool{Type: "function", Function: model.ToolFunction{Name: "highlight"}},
				Execute:    SyntheticResult("success"),
			},
			{
				Definition: model.Tool{Type: "function", Function: model.ToolFunction{Name: "jump"}},
				Execute: func(context.Context, string) (string, error) {
					return "", errors.New("目标不存在")
				},
			},
		},
		Handler: h,
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		"assistant(tool_calls)",
		"tool(highlight:success)",
		"tool(jump:工具执行失败: 目标不存在)",
		"assistant(text:讲完了)",
	}, s.seq)
	require.Len(t, s.called, 1)
	require.Len(t, s.called[0], 2)

	// 第二轮请求应携带成对的 assistant + tool 消息。
	require.Len(t, c.got, 2)
	second := c.got[1].Messages
	require.Len(t, second, 3)
	require.Equal(t, model.RoleAssistant, second[0].Role)
	require.Len(t, second[0].ToolCalls, 2)
	require.Equal(t, model.RoleTool, second[1].Role)
	require.Equal(t, "c1", second[1].ToolCallID)
	require.Equal(t, model.RoleTool, second[2].Role)
	require.Equal(t, "c2", second[2].ToolCallID)
}

// 未知工具不中断，回传错误文本。
func TestRunUnknownTool(t *testing.T) {
	c := &scriptClient{turns: []scriptTurn{
		{calls: []model.ToolCall{{ID: "c9", Function: model.ToolCallFunc{Name: "ghost"}}}},
		{deltas: []string{"ok"}},
	}}
	h, s := collect()
	err := newRunner(c, 0).Run(context.Background(), Request{
		Handler: h,
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		"assistant(tool_calls)",
		"tool(ghost:工具执行失败: 未知工具 ghost)",
		"assistant(text:ok)",
	}, s.seq)
}

// 到达轮次上限：末轮去掉 Tools 并注入收尾 system 提示词，必为文本收尾。
func TestRunMaxTurnsForceFinal(t *testing.T) {
	c := &scriptClient{turns: []scriptTurn{
		{calls: []model.ToolCall{{ID: "c1", Function: model.ToolCallFunc{Name: "draw"}}}},
		{calls: []model.ToolCall{{ID: "c2", Function: model.ToolCallFunc{Name: "draw"}}}},
		{deltas: []string{"收尾"}},
	}}
	h, s := collect()
	drawCalls := 0
	err := newRunner(c, 3).Run(context.Background(), Request{
		Tools: []Tool{{
			Definition: model.Tool{Type: "function", Function: model.ToolFunction{Name: "draw"}},
			Execute: func(context.Context, string) (string, error) {
				drawCalls++
				return "success", nil
			},
		}},
		Handler: h,
	})
	require.NoError(t, err)
	require.Equal(t, 2, drawCalls)
	require.Equal(t, "收尾", s.texts)
	require.Len(t, c.got, 3)
	final := c.got[2]
	require.Nil(t, final.Tools, "末轮不应下发任何工具")
	require.Len(t, final.Messages, 5)
	sys := final.Messages[len(final.Messages)-1]
	require.Equal(t, model.RoleSystem, sys.Role)
	require.NotEmpty(t, sys.Content, "应注入强制收尾提示词")
}

// OnMessage 返回的 error 应中断 loop 并原样上抛。
func TestRunHandlerInterrupt(t *testing.T) {
	sentinel := errors.New("stop")
	c := &scriptClient{turns: []scriptTurn{{deltas: []string{"x"}}}}
	err := newRunner(c, 0).Run(context.Background(), Request{
		Handler: Handler{OnMessage: func(context.Context, model.ChatMessage) error {
			return sentinel
		}},
	})
	require.ErrorIs(t, err, sentinel)
}
