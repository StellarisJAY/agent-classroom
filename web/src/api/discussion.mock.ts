import type { DiscussionSSEEvent } from './discussion'

/**
 * 讨论流 mock：与真实后端同协议（SSE 帧 + {type:text|action|end}），
 * 以 ReadableStream 模拟响应体，保证前端执行器/队列逻辑可脱离后端联调。
 * 后端就位后删除该文件并让 askQuestion 直连真实实现。
 */
export const DISCUSSION_MOCK = true

/** 组装一段"文本 → 动作 → 文本"的模拟回复（覆盖全部动作类型一轮）。 */
export function buildMockScript(sectionId: string | null): string[] {
  const frames: string[] = []
  const push = (e: DiscussionSSEEvent) => frames.push(`data: ${JSON.stringify(e)}\n\n`)

  push({ type: 'text', delta: '好的，我们来看这里。' })
  if (sectionId) {
    push({ type: 'action', action: { type: 'highlight', targetElementId: 'el-title' } })
    push({ type: 'action', action: { type: 'laser' } })
    push({ type: 'action', action: { type: 'draw', drawing: {
      kind: 'arrow', color: '#f97316', size: 'thick', points: [[240, 120], [420, 220]],
    } } })
  }
  push({ type: 'text', delta: '这个要素的意义在于把抽象的定义落到具体的示例上。\n' })
  push({ type: 'action', action: { type: 'clear_board' } })
  push({ type: 'text', delta: '还有其他问题吗？' })
  push({ type: 'end' })
  return frames
}

/** 把帧序列变成模拟 SSE 响应体（帧间隔 120ms，模拟真实流节奏）。 */
export function mockBody(frames: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder()
  return new ReadableStream<Uint8Array>({
    async start(controller) {
      for (const f of frames) {
        await new Promise((r) => setTimeout(r, 120))
        controller.enqueue(enc.encode(f))
      }
      controller.close()
    },
  })
}
