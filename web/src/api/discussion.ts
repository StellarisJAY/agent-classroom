import type { SlideAction, SlideDrawing } from './learn'
import { request } from './http'
import { ApiError } from './error'
import { consumePostSSE, type SSEFrame } from './sse'

/**
 * 讨论模式接口：POST /courses/:id/questions 建立 SSE 单流，
 * 后端在流内完成整个 agent loop（轮内本地合成工具结果）。
 * 事件协议见 docs/讨论模式方案.md §8 与 internal/handler/sse.go 的 sseEvent：
 * {type:"text",delta} / {type:"action",name,args} / {type:"end"} / {type:"error",msg}。
 */

// ---- 动作类型（与后端工具 schema 对齐：discussion_tools.go）----

/** 动作名 → 语义动作的映射；前端是动作执行器，此处只做协议转换与降级。 */

export interface JumpAction {
  type: 'jump_to_section'
  section_id: string
}

export interface ElementAction extends Omit<SlideAction, 'drawing'> {
  type: 'underline' | 'highlight' | 'box' | 'laser'
}

export interface DrawAction {
  type: 'draw'
  drawing: SlideDrawing
}

export interface ClearBoardAction {
  type: 'clear_board'
}

export type DiscussionAction = JumpAction | ElementAction | DrawAction | ClearBoardAction

// ---- SSE 事件 ----

export type DiscussionSSEEvent =
  | { type: 'text'; delta: string }
  | { type: 'action'; name: string; args: unknown }
  | { type: 'end' }
  | { type: 'error'; msg: string }

/** 后端 action 事件 args 的宽松形状。 */
interface ActionArgs {
  section_id?: unknown
  step_index?: unknown
  element_id?: unknown
  x?: unknown
  y?: unknown
  drawing?: unknown
}

interface RawActionEvent {
  type?: unknown
  delta?: unknown
  name?: unknown
  args?: unknown
  msg?: unknown
}

/** 把 action 事件 {name, args} 映射为语义动作；未知工具或残缺参数返回 null（静默降级）。 */
export function mapAction(name: string, args: unknown): DiscussionAction | null {
  const a = (typeof args === 'object' && args !== null ? args : {}) as ActionArgs
  switch (name) {
    case 'jump_to_section':
      return typeof a.section_id === 'string' && a.section_id
        ? { type: 'jump_to_section', section_id: a.section_id }
        : null
    case 'highlight':
    case 'underline':
    case 'box': {
      if (typeof a.element_id !== 'string' || !a.element_id) return null
      return { type: name, targetElementId: a.element_id } as ElementAction
    }
    case 'laser': {
      // laser：引用元素 id 或画布坐标至少其一
      if (typeof a.element_id === 'string' && a.element_id) {
        return { type: 'laser', targetElementId: a.element_id } as ElementAction
      }
      if (typeof a.x === 'number' && typeof a.y === 'number') {
        return { type: 'laser', x: a.x, y: a.y } as ElementAction
      }
      return null
    }
    case 'draw': {
      if (a.drawing === null || typeof a.drawing !== 'object') return null
      const drawing = a.drawing as SlideDrawing
      if (typeof drawing?.kind !== 'string') return null
      return { type: 'draw', drawing }
    }
    case 'clear_board':
      return { type: 'clear_board' }
    default:
      return null
  }
}

export function parseDiscussionEvent(frame: SSEFrame): DiscussionSSEEvent | null {
  if (!frame.data || frame.data === '[DONE]') return null
  let e: RawActionEvent
  try {
    e = JSON.parse(frame.data) as RawActionEvent
  } catch {
    return null
  }
  switch (e.type) {
    case 'text':
      return typeof e.delta === 'string' ? { type: 'text', delta: e.delta } : null
    case 'action':
      return typeof e.name === 'string' && e.name
        ? { type: 'action', name: e.name, args: e.args }
        : null
    case 'end':
      return { type: 'end' }
    case 'error':
      return typeof e.msg === 'string' ? { type: 'error', msg: e.msg } : null
    default:
      return null
  }
}

// ---- 提问（SSE 单流） ----

export interface AskDiscussionInput {
  question: string
  /** 提问来源环节，可空（脱离环节的全局提问） */
  section_id?: string | null
  /** 提问时所处的讲解步骤下标（slide 环节才有意义，可空） */
  step_index?: number | null
  /** 目标会话，可空（不传则后端隐式新建会话，即"新对话"） */
  conversation_id?: string | null
}

export interface StreamDiscussionHandlers {
  onText: (delta: string) => void
  onAction: (action: DiscussionAction) => void
  onEnd: () => void
  onError: (err: unknown) => void
}

/** 后端 SSE 通路（/api/courses/:id/questions）。 */
export async function askQuestion(
  courseId: string,
  input: AskDiscussionInput,
  h: StreamDiscussionHandlers,
  signal: AbortSignal,
): Promise<void> {
  const dispatch = (frame: SSEFrame) => {
    const e = parseDiscussionEvent(frame)
    if (!e) return
    switch (e.type) {
      case 'text':
        h.onText(e.delta)
        break
      case 'action': {
        const action = mapAction(e.name, e.args)
        if (action) h.onAction(action) // 映射失败静默降级（方案 §2 边界 2）
        break
      }
      case 'error':
        h.onError(new ApiError(50000, e.msg))
        break
      default:
        h.onEnd()
    }
  }
  try {
    await consumePostSSE(`/api/courses/${courseId}/questions`, {
      body: {
        question: input.question,
        section_id: input.section_id ?? null,
        step_index: input.step_index ?? null,
        conversation_id: input.conversation_id ?? null,
      },
      signal,
      onEvent: dispatch,
    })
    h.onEnd() // 服务端已显式 onEnd 时幂等（由调用方去重）
  } catch (e) {
    h.onError(e instanceof ApiError ? e : new ApiError(50000, '讨论流中断，请重新提问'))
  }
}

// ---- 会话列表与问答历史 ----

/** 会话列表项（GET /courses/:id/conversations）。 */
export interface ConversationSummary {
  id: string
  title: string
  update_at: string
}

/** 拉取课程下会话列表（按最近活跃倒序）。 */
export async function listConversations(courseId: string): Promise<ConversationSummary[]> {
  return request<ConversationSummary[]>({
    url: `/courses/${courseId}/conversations`,
    method: 'get',
  })
}

/** 后端结构化消息：content 为按 role 区分的 jsonb（见 types/conversation.go MessageContent）。 */
export interface ConversationMessage {
  id: string
  role: 'user' | 'assistant' | 'tool'
  content: ConversationMessageContent
  section_id?: string | null
  created_at: string
}

export interface ConversationMessageContent {
  text?: string
  tool_calls?: { id: string; name: string; arguments: string }[]
  tool_call_id?: string
  name?: string
  result?: string
}

/** 拉取一处会话的问答历史（结构化消息，前端自行适配）；
 * conversation_id 缺省时返回最近活跃会话的历史。 */
export async function listConversation(
  courseId: string,
  conversationId?: string | null,
): Promise<ConversationMessage[]> {
  return request<ConversationMessage[]>({
    url: `/courses/${courseId}/conversation`,
    method: 'get',
    params: conversationId ? { conversation_id: conversationId } : undefined,
  })
}
