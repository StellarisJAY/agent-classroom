import { describe, expect, it } from 'vitest'

import type { SSEFrame } from '@/api/sse'
import { readEventFrames } from '@/api/sse'

function streamOf(chunks: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder()
  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const c of chunks) controller.enqueue(enc.encode(c))
      controller.close()
    },
  })
}

describe('readEventFrames（SSE 手工解析）', () => {
  it('跨 chunk 事件缓冲与按序解析', async () => {
    const frames: SSEFrame[] = []
    await readEventFrames(
      streamOf(['data: {"type":"te', 'xt","delta":"你好"}\n\ndata: {"type":"end"}\n\n']),
      (f) => frames.push(f),
    )
    expect(frames.map((f) => JSON.parse(f.data))).toEqual([
      { type: 'text', delta: '你好' },
      { type: 'end' },
    ])
  })

  it('event: 行与 guessable data 多行拼接', async () => {
    const frames: SSEFrame[] = []
    await readEventFrames(
      streamOf(['event: action\ndata: {"type":"action"}\n\n']),
      (f) => frames.push(f),
    )
    expect(frames).toHaveLength(1)
    expect(frames[0]!.event).toBe('action')
  })

  it('帧间残留缓冲不影响后续完整帧', async () => {
    const frames: SSEFrame[] = []
    await readEventFrames(
      streamOf(['data: a\ndata: {"x":1}\n\ndata: {"y":2}\n\n']),
      (f) => frames.push(f),
    )
    // 第一帧多行 data 拼接后非 JSON，由业务层 parse 兜底忽略
    expect(frames[0]!.event).toBe('message')
    expect(JSON.parse(frames[1]!.data)).toEqual({ y: 2 })
  })
})
