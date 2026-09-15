import type { SlideAction, SlideDrawing } from './learn'
import { ApiError } from './error'
import { consumePostSSE, readEventFrames, type SSEFrame } from './sse'
import { DISCUSSION_MOCK, buildMockScript, mockBody } from './discussion.mock'

/**
 * 讨论模式接口：POST a SSE 单流完成整个 agent loop（后端轮内合成工具结果）。
 * 事件协议见 docs/讨论模式方案.md §8：{type:"text"|action|end}。
 */

/** 讨论动作：与 SlideAction 对齐（结对元素动作），另加 jump/clear_board 语义动作。 */
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

export type DiscussionSSEEvent =
  | { type: 'text'; delta: string }
  | { type: 'action'; action: DiscussionAction }
  | { type: 'end' }

export interface AskDiscussionInput {
  content: string
  /** 提问来源环节，可空 */
  section_id?: string | null
}

export function parseDiscussionEvent(frame: SSEFrame): DiscussionSSEEvent | null {
  if (!frame.data || frame.data === '[DONE]') return null
  const e = JSON.parse(frame.data) as DiscussionSSEEvent
  if (e?.type !== 'text' && e?.type !== 'action' && e?.type !== 'end') return null
  return e
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
    if (e.type === 'text') h.onText(e.delta)
    else if (e.type === 'action') h.onAction(e.action)
    else h.onEnd()
  }
  try {
    if (DISCUSSION_MOCK) {
      // 假流：协议与真实后端一致（帧间隔模拟流节奏），仅替换数据源
      await readEventFrames(mockBody(buildMockScript(input.section_id ?? null)), dispatch)
    } else {
      await consumePostSSE(`/api/courses/${courseId}/questions`, {
        body: { content: input.content, section_id: input.section_id ?? null },
        signal,
        onEvent: dispatch,
      })
    }
    h.onEnd() // 服务端已显式 onEnd 时幂等（由调用方去重）
  } catch (e) {
    h.onError(e instanceof ApiError ? e : new ApiError(50000, '讨论流中断，请重新提问'))
  }
}
