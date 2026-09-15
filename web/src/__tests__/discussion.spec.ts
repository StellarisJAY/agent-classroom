import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { DiscussionAction, DiscussionSSEEvent } from '@/api/discussion'
import type { CourseLearnDetail } from '@/api/learn'
import { useConversationStore } from '@/stores/conversation'
import { useDiscussionStore } from '@/stores/discussion'
import { useLearnStore } from '@/stores/learn'

// 讨论流 mock：由用例注入脚本（逐帧回调驱动 discussion store 的严格队列）
type StreamHandlers = {
  onText: (d: string) => void
  onAction: (a: DiscussionAction) => void
  onEnd: () => void
  onError: (e: unknown) => void
}

const askQuestion = vi.fn<(cid: string, input: unknown, h: StreamHandlers, signal: AbortSignal) => Promise<void>>(
  async () => {},
)
vi.mock('@/api/discussion', () => ({
  askQuestion: (...args: Parameters<typeof askQuestion>) => askQuestion(...args),
}))

function scriptEvents(events: DiscussionSSEEvent[]) {
  askQuestion.mockImplementation(async (_cid, _input, h, signal) => {
    for (const e of events) {
      if (signal.aborted) return
      if (e.type === 'text') h.onText(e.delta)
      else if (e.type === 'action') h.onAction(e.action)
      else h.onEnd()
    }
  })
}

function slideDetail(): CourseLearnDetail {
  return {
    course: { id: 'c1', title: '数组' },
    progress: 'in_progress' as const,
    sections: [
      {
        id: 'sec-1',
        position: 0,
        type: 'slide' as const,
        title: '声明',
        knowledge_points: [],
        status: 'done' as const,
        content: { width: 1280, height: 720, background: '#fff', accent: '#14b8a6', elements: [] } as never,
        steps: [
          { text: '步骤一' },
          { text: '步骤二' },
        ] as never,
        questions: [],
      },
      {
        id: 'sec-2',
        position: 1,
        type: 'quiz' as const,
        title: '测验',
        knowledge_points: [],
        status: 'done' as const,
        content: null,
        steps: null,
        questions: [],
      },
    ],
  }
}

describe('discussion store（讨论模式状态机）', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('进入:快照 + 锁定导航 + 暂停播放；快照位置在退出后恢复', async () => {
    const learn = useLearnStore()
    learn.detail = slideDetail()
    const chat = useConversationStore()
    const discussion = useDiscussionStore()
    discussion.init('c1')

    // 进入讨论：位于环节 0 步骤 1
    learn.goTo(0)
    learn.stepIndex = 1
    askQuestion.mockImplementation(async () => {}) // 等待中的流

    discussion.start('为什么数组下标从 0 开始？')
    expect(discussion.active).toBe(true)
    expect(learn.sectionLocked).toBe(true)
    expect(learn.autoPlaying).toBe(false)
    expect(chat.messages[chat.messages.length - 1]?.role).toBe('user')
    expect(chat.messages[chat.messages.length - 1]?.content).toBe('为什么数组下标从 0 开始？')

    // 手动导航被锁
    expect(await learn.goTo(1)).toBe(false)
    learn.nextStep()
    expect(learn.stepIndex).toBe(1) // 锁定后 nextStep 失效

    // 退出恢复快照 + 解锁
    discussion.stop()
    expect(discussion.active).toBe(false)
    expect(learn.sectionLocked).toBe(false)
    expect(learn.currentIndex).toBe(0)
    expect(learn.stepIndex).toBe(1)
  })

  it('严格队列：action 全部落地后才渲染后续文本；jump 可解锁通道移动位置', async () => {
    const learn = useLearnStore()
    learn.detail = slideDetail()
    const chat = useConversationStore()
    const discussion = useDiscussionStore()
    discussion.init('c1')

    scriptEvents([
      { type: 'text', delta: '先看' },
      { type: 'action', action: { type: 'highlight', targetElementId: 'el-1' } },
      { type: 'action', action: { type: 'jump_to_section', section_id: 'sec-2' } },
      { type: 'text', delta: '后讲' },
      { type: 'end' },
    ])

    discussion.start('讲讲这里')
    await discussion.drain()

    // 动作已全部落地
    expect(learn.currentIndex).toBe(1) // jump 带走位置
    expect(learn.sectionLocked).toBe(true) // 仍锁定
    const focus = discussion.overlayActions
    expect(focus.map((a) => a.type)).toContain('highlight')
    expect(discussion.actionLog.map((a) => a.type)).toEqual(['highlight', 'jump_to_section'])

    // 本轮结束后讨论仍激活（覆盖层保留），但流式结束
    expect(discussion.streaming).toBe(false)
    expect(discussion.active).toBe(true)
    // 完整旁白落为一条 assistant 消息
    const assistant = chat.messages.filter((m) => m.role === 'assistant')
    expect(assistant.map((m) => m.content)).toEqual(['先看后讲'])
  })

  it('end 后 overlayActions 保留、stop 清叠加层并恢复快照', async () => {
    const learn = useLearnStore()
    learn.detail = slideDetail()
    const discussion = useDiscussionStore()
    discussion.init('c1')
    learn.goTo(0)
    learn.stepIndex = 1

    scriptEvents([
      { type: 'action', action: { type: 'draw', drawing: { kind: 'pen', points: [[10, 10]] } } },
      { type: 'end' },
    ])
    discussion.start('画一下')
    await discussion.drain()
    expect(discussion.overlayStrokes).toHaveLength(1)

    discussion.stop()
    expect(discussion.overlayStrokes).toHaveLength(0)
    expect(learn.sectionLocked).toBe(false)
    expect(learn.currentIndex).toBe(0)
  })

  it('讨论叠加笔画按当前环节画布归一化（pen 点放大到 1280×720）', async () => {
    const learn = useLearnStore()
    const detail = slideDetail()
    // 产出画布 640×720 → x 缩放 ×2、y 不缩放
    detail.sections[0]!.content = { width: 640, height: 720, background: '#fff', accent: '#14b8a6', elements: [] } as never
    learn.detail = detail
    const discussion = useDiscussionStore()
    discussion.init('c1')

    scriptEvents([
      { type: 'action', action: { type: 'draw', drawing: { kind: 'pen', points: [[100, 200]] } } },
      { type: 'end' },
    ])
    discussion.start('看这')
    await discussion.drain()
    expect(discussion.overlayStrokes[0]?.drawing.points?.[0]).toEqual([200, 200])
  })
})
