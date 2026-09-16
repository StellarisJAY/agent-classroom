import { describe, expect, it } from 'vitest'

import {
  mapAction,
  parseDiscussionEvent,
  type DiscussionSSEEvent,
} from '@/api/discussion'
import { readEventFrames, type SSEFrame } from '@/api/sse'

// SSE 事件协议解析（对齐后端 internal/handler/sse.go sseEvent）
describe('parseDiscussionEvent', () => {
  it('解析 text / action / end / error 四类事件', () => {
    expect(parseDiscussionEvent({ event: 'message', data: JSON.stringify({ type: 'text', delta: '好的' }) }))
      .toEqual({ type: 'text', delta: '好的' })
    expect(parseDiscussionEvent({ event: 'message', data: JSON.stringify({ type: 'action', name: 'highlight', args: { element_id: 'e1' } }) }))
      .toEqual({ type: 'action', name: 'highlight', args: { element_id: 'e1' } })
    expect(parseDiscussionEvent({ event: 'message', data: JSON.stringify({ type: 'end' }) }))
      .toEqual({ type: 'end' })
    expect(parseDiscussionEvent({ event: 'message', data: JSON.stringify({ type: 'error', msg: '模型调用失败' }) }))
      .toEqual({ type: 'error', msg: '模型调用失败' })
  })

  it('过滤 [DONE] / 空帧 / 非法 JSON / 残缺字段', () => {
    expect(parseDiscussionEvent({ event: 'message', data: '[DONE]' })).toBeNull()
    expect(parseDiscussionEvent({ event: 'message', data: '' })).toBeNull()
    expect(parseDiscussionEvent({ event: 'message', data: 'not-json' })).toBeNull()
    expect(parseDiscussionEvent({ event: 'message', data: JSON.stringify({ type: 'text' }) })).toBeNull()
    expect(parseDiscussionEvent({ event: 'message', data: JSON.stringify({ type: 'action', args: { element_id: 'e1' } }) })).toBeNull()
    expect(parseDiscussionEvent({ event: 'message', data: JSON.stringify({ type: 'error' }) })).toBeNull()
    expect(parseDiscussionEvent({ event: 'message', data: JSON.stringify({ type: 'unknown' }) })).toBeNull()
  })
})

// 工具调用名+参数 → 前端语义动作（对齐后端 discussion_tools.go schema）
describe('mapAction', () => {
  it('元素强调：jump/highlight/underline/box', () => {
    expect(mapAction('jump_to_section', { section_id: 'sec-2', step_index: 3 }))
      .toEqual({ type: 'jump_to_section', section_id: 'sec-2' })
    expect(mapAction('highlight', { element_id: 'e1' }))
      .toEqual({ type: 'highlight', targetElementId: 'e1' })
    expect(mapAction('underline', { element_id: 'e2' }))
      .toEqual({ type: 'underline', targetElementId: 'e2' })
    expect(mapAction('box', { element_id: 'e3' }))
      .toEqual({ type: 'box', targetElementId: 'e3' })
  })

  it('laser：元素 id 与坐标两种形态都接受，缺一返回 null', () => {
    expect(mapAction('laser', { element_id: 'e1' }))
      .toEqual({ type: 'laser', targetElementId: 'e1' })
    expect(mapAction('laser', { x: 120, y: 80 }))
      .toEqual({ type: 'laser', x: 120, y: 80 })
    expect(mapAction('laser', {})).toBeNull()
  })

  it('draw：透传 drawing；kind 缺失降级丢弃', () => {
    const drawing = { kind: 'pen', points: [[10, 20]] } as const
    expect(mapAction('draw', { drawing })).toEqual({ type: 'draw', drawing })
    expect(mapAction('draw', {})).toBeNull()
    expect(mapAction('draw', { drawing: { points: [] } })).toBeNull()
  })

  it('clear_board 与未知工具', () => {
    expect(mapAction('clear_board', {})).toEqual({ type: 'clear_board' })
    expect(mapAction('mystery_tool', { element_id: 'x' })).toBeNull()
  })
})

// 真实解析管线（readEventFrames）→ 语义事件的端到端顺序
describe('askQuestion 事件流（readEventFrames 管线）', () => {
  it('帧序列按序派发 text/action/end，error 走 onError', async () => {
    const payload: DiscussionSSEEvent[] = [
      { type: 'text', delta: '好' },
      { type: 'action', name: 'highlight', args: { element_id: 'e1' } },
      { type: 'text', delta: '再看' },
      { type: 'error', msg: '中断' },
    ]
    const enc = new TextEncoder()
    const body = new ReadableStream<Uint8Array>({
      start(c) {
        for (const e of payload) c.enqueue(enc.encode(`data: ${JSON.stringify(e)}\n\n`))
        c.close()
      },
    })
    const seen: string[] = []
    await readEventFrames(body, (f: SSEFrame) => {
      const e = parseDiscussionEvent(f)
      if (e) seen.push(e.type)
    })
    expect(seen).toEqual(['text', 'action', 'text', 'error'])
  })
})
