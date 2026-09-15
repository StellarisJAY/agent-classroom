# Agent Loop · 统一封装规范

> 状态：已定案并实施于 `internal/agent`。
>
> 本文档定义项目内统一的 agent loop 封装：任何需要「模型多轮驱动 + 工具调用」的功能
> （如讨论模式的智能体老师）一律经此封装驱动，禁止各业务自行手写循环。

## 1. 定位与依赖

- 包位置：`internal/agent`，仅依赖 `internal/model`（协议类型），不依赖 types/repo/service。
- 业务自下而上装配：`model.Registry（或直接 client）+ Runner + 业务 Request/Handler`。
- 底层流式协议由 `model.ToolStreamClient.ChatToolStream`（手写 OpenAI 适配器）提供。

## 2. 核心类型

| 类型 | 说明 |
|---|---|
| `Tool` | 一个可用工具 = OpenAI 协议定义（`model.Tool`）+ 执行器 `Execute(ctx, argsJSON) (result, err)`；argsJSON 为模型 arguments 原始 JSON 串，不解析透传 |
| `SyntheticResult(result)` | 便捷执行器：忽略参数合成固定结果（讨论模式动作类工具直接用） |
| `Handler` | 事件分发：`OnText`（文本增量）、`OnToolCall`（本轮完整调用集合，一次）、`OnMessage`（每条产出消息，用于逐条落库）；均可 nil，返回 error 中断 loop |
| `Runner` | 无状态运行器（`Client` + `MaxTurns`，默认 8），可并发复用 |
| `Request` | `Messages`（业务装配的初始上下文）、`Tools`（业务裁剪后的工具集）、`Handler`、`Temperature` |

**上下文注入约定**：loop 完全不参与 system / 业务上下文 / 历史窗口的拼装——初始
`Messages` 由业务提供（如讨论模式 service 的 quiz 剔除答案、48k 字符 LRU 截断都留在
业务装配函数内）。loop 只负责轮间追加（assistant tool_call ↔ tool 结果成对）与收尾。
工具集按场景裁剪同样是业务职责（如 demo 环节仅注入 `jump_to_section`）。

## 3. 循环语义（Run）

1. 每轮调用 `ChatToolStream`：文本增量逐片经 `OnText` 上抛；工具调用按 index 拼装
   完成后经 `OnToolCall` 上抛。
2. 消息追加顺序（每轮）：assistant(tool_calls) → **先** `OnMessage`（落库时序优先）
   **再** `OnToolCall`（SSE 动作），随后逐条执行工具、追加 tool 结果并 `OnMessage`。
3. 纯文本轮即为收尾：assistant 文本消息 `OnMessage` 后 loop 结束。
4. **轮次上限强制收尾**：到达 `MaxTurns` 仍未收尾时，注入系统提示词
   （`prompts/force_final.md`，go:embed）且**最后一轮不下发任何 Tools**（物理上杜绝
   继续调用工具），必为纯文本收尾。
5. **工具失败不中断**：执行器返回 error 时，以 `工具执行失败: <err>` 文本作为该次
   调用的 tool 结果回传模型（模型可自我纠错）；未知工具同样回传错误文本。
6. 任一 Handler 回调返回 error → loop 立即中断并原样上抛；context 取消随 LLM 调用
   自然传播。

## 4. 使用示例（以讨论模式为例）

```go
runner := &agent.Runner{Client: client} // client 经 model.Registry 构造
err := runner.Run(ctx, agent.Request{
    Messages: discussionMessages, // system 人设 + 环节上下文 + LRU 历史 + 本轮提问
    Tools: []agent.Tool{
        {Definition: highlightDef, Execute: agent.SyntheticResult("success")},
        {Definition: jumpDef, Execute: doJump}, // 需要真实执行的工具由业务实现
    },
    Handler: agent.Handler{
        OnText:    sseEmitText,   // → SSE {type:"text", delta}
        OnToolCall: sseEmitAction, // → SSE {type:"action", action}
        OnMessage: persist,       // 逐条落库 user/assistant/tool 消息
    },
})
```

## 5. 测试约定

- 单测用脚本化假客户端（实现 `model.ToolStreamClient`，按轮次播放 delta/calls 并断言
  收到的请求），覆盖：纯文本收尾、成对消息回传、未知/失败工具回传、轮次上限末轮
  去除 Tools + 注入收尾提示词、Handler 中断。
- 涉及真实 SSE 协议拼装的测试见 `internal/model/llm/openai_test.go`（分层不重复）。
