// Package agent 提供与上层业务无关的统一 agent loop 封装：
// 驱动「LLM 流式输出 → 工具调用 → 回传工具结果 → 下一轮」的循环，
// 上层通过 Request 注入业务上下文与工具集，通过 Handler 消费流式事件与
// 逐条落库消息。当前默认实现走 OpenAI 兼容协议（model.ToolStreamClient）。
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/StellarisJAY/agent-classroom/internal/model"
)

// DefaultMaxTurns 单次运行的默认轮次上限（每个轮次 = 一次 LLM 调用）。
const DefaultMaxTurns = 8

// Tool 一个可用工具 = OpenAI 协议的工具定义 + 执行器。
// Execute 的 argsJSON 为模型给出的 arguments 原始 JSON 字符串（不解析透传），
// 返回字符串将作为 tool 结果消息回传模型。
type Tool struct {
	Definition model.Tool
	Execute    func(ctx context.Context, argsJSON string) (result string, err error)
}

// SyntheticResult 返回一个合成固定结果、忽略参数的执行器。
// 适用于动作类工具（前端执行、后端不感知成败，如讨论模式）。
func SyntheticResult(result string) func(context.Context, string) (string, error) {
	return func(context.Context, string) (string, error) {
		return result, nil
	}
}

// Handler 流式事件与消息分发。全部字段可为 nil（对应回调跳过）；
// 任一回调返回 error 会中断整个 loop 并作为 Run 的返回值上抛。
type Handler struct {
	// OnText 文本增量回调（每片一次）。
	OnText func(ctx context.Context, delta string) error
	// OnToolCall 收到本轮完整工具调用集合时回调一次（纯文本轮不触发）。
	// 用于向演示层上抛动作（如讨论模式的 SSE action 事件）。
	OnToolCall func(ctx context.Context, calls []model.ToolCall) error
	// OnMessage 每产生一条新消息（assistant 文本 / assistant 工具调用 /
	// tool 结果）回调一次。用于业务侧逐条即时落库等持久化诉求。
	OnMessage func(ctx context.Context, msg model.ChatMessage) error
}

// Runner agent loop 运行器。仅持有不可变的客户端引用，可并发复用。
type Runner struct {
	// Client 工具流式客户端（OpenAI 适配器已实现）。
	Client model.ToolStreamClient
	// MaxTurns 轮次上限；<=0 取 DefaultMaxTurns。
	MaxTurns int
}

// Request 一次 agent 运行的输入。初始上下文完全由业务装配
// （system 人设 + 业务上下文注入 + 历史 LRU 截断 + 本轮 user 消息），
// loop 只负责轮间的消息追加与收尾，不感知任何业务语义。
type Request struct {
	// Messages 初始消息序列（业务装配）。
	Messages []model.ChatMessage
	// Tools 本次运行可用的工具集；业务负责按场景裁剪（如 demo 环节仅保留切换环节）。
	Tools []Tool
	// Handler 事件与消息分发器。
	Handler Handler
	// Temperature 采样温度；nil 用模型默认。
	Temperature *float64
	// Thinking 思考限制（off / default / max；空串视为 default）。透传至模型请求。
	Thinking string
}

// Run 驱动完整 agent loop：
//
//   - 每轮调用 Client.ChatToolStream：文本增量经 Handler.OnText 上抛，
//     工具调用拼装完成后经 Handler.OnToolCall 上抛，并按序追加
//     assistant(tool_calls) 与各 tool 结果消息（经 Handler.OnMessage 回调）。
//   - 纯文本轮视为收尾：assistant 文本消息经 OnMessage 回调后结束。
//   - 顺序红线：assistant tool_call 消息先 OnMessage（落库），
//     再 OnToolCall（SSE 动作），与讨论模式方案的落库时序对齐。
//   - 到达轮次上限仍未收尾：注入强制收尾 system 提示词，
//     且最后一轮不再下发 Tools（物理上杜绝继续调用工具），必为纯文本收尾。
//   - 工具执行失败不中断 loop：错误信息文本作为该次调用的 tool 结果回传。
func (r *Runner) Run(ctx context.Context, req Request) error {
	maxTurns := r.MaxTurns
	if maxTurns <= 0 {
		maxTurns = DefaultMaxTurns
	}
	tools := append([]Tool(nil), req.Tools...)
	msgs := append([]model.ChatMessage(nil), req.Messages...)

	for turn := 1; turn <= maxTurns; turn++ {
		// 最后一轮强制收尾：注入收尾 system 提示词，且不下发任何工具。
		final := turn == maxTurns
		turnMsgs, turnTools := msgs, toolDefs(tools)
		if final {
			turnMsgs = append(append([]model.ChatMessage(nil), msgs...),
				model.ChatMessage{Role: model.RoleSystem, Content: forceFinalText()})
			turnTools = nil
		}

		var text strings.Builder
		var calls []model.ToolCall
		err := r.Client.ChatToolStream(ctx, model.ChatRequest{
			Messages:    turnMsgs,
			Tools:       turnTools,
			Temperature: req.Temperature,
			Thinking:    req.Thinking,
		}, model.StreamHandler{
			OnText: func(delta string) error {
				text.WriteString(delta)
				if req.Handler.OnText != nil {
					return req.Handler.OnText(ctx, delta)
				}
				return nil
			},
			OnToolCall: func(ts []model.ToolCall) error {
				calls = append(calls, ts...)
				return nil
			},
		})
		if err != nil {
			return fmt.Errorf("agent loop 第 %d 轮: %w", turn, err)
		}

		if len(calls) == 0 {
			return emitMessage(ctx, req.Handler.OnMessage,
				model.ChatMessage{Role: model.RoleAssistant, Content: text.String()})
		}

		// 落库时序优先于动作分发：assistant(tool_calls) 先持久化再上抛动作。
		am := model.ChatMessage{Role: model.RoleAssistant, ToolCalls: calls}
		msgs = append(msgs, am)
		if err := emitMessage(ctx, req.Handler.OnMessage, am); err != nil {
			return err
		}
		if req.Handler.OnToolCall != nil {
			if err := req.Handler.OnToolCall(ctx, calls); err != nil {
				return err
			}
		}

		// 执行工具并按协议成对追加 tool 结果消息（多调用逐条）。
		for _, call := range calls {
			tm := model.ChatMessage{
				Role:       model.RoleTool,
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    execTool(ctx, tools, call),
			}
			msgs = append(msgs, tm)
			if err := emitMessage(ctx, req.Handler.OnMessage, tm); err != nil {
				return err
			}
		}
	}
	return nil
}

// toolDefs 抽取协议侧工具定义。
func toolDefs(tools []Tool) []model.Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]model.Tool, len(tools))
	for i, t := range tools {
		out[i] = t.Definition
	}
	return out
}

// execTool 按名称执行工具；未知工具与执行失败均以错误文本作为结果回传，不中断 loop。
func execTool(ctx context.Context, tools []Tool, call model.ToolCall) string {
	for _, t := range tools {
		if t.Definition.Function.Name != call.Function.Name {
			continue
		}
		result, err := t.Execute(ctx, call.Function.Arguments)
		if err != nil {
			return fmt.Sprintf("工具执行失败: %v", err)
		}
		if result == "" {
			return "success"
		}
		return result
	}
	return fmt.Sprintf("工具执行失败: 未知工具 %s", call.Function.Name)
}

// emitMessage 经 OnMessage 回调一条消息；回调未设置则直通。
func emitMessage(ctx context.Context, on func(context.Context, model.ChatMessage) error, msg model.ChatMessage) error {
	if on == nil {
		return nil
	}
	return on(ctx, msg)
}
