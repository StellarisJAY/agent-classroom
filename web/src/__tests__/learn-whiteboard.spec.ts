import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useLearnStore } from '@/stores/learn'
import type { SectionLearn, SlideContent } from '@/api/learn'

// 白板回放与视图派生逻辑测试：gesture 动作（draw / clearBoard / switchView / laser）。
vi.mock('@/api/learn', () => ({
  getCourseDetail: vi.fn<() => Promise<unknown>>(),
  updateProgress: vi.fn<() => Promise<unknown>>().mockResolvedValue(undefined),
  saveDemoCode: vi.fn<() => Promise<unknown>>(),
  isDemoType: (t: string) => t.startsWith('demo'),
  getConversation: vi.fn<() => Promise<unknown>>(),
}))

function slideSection(
  id: string,
  steps: SectionLearn['steps'],
): SectionLearn {
  const content: SlideContent = {
    width: 1280,
    height: 720,
    background: '#ffffff',
    accent: '#14b8a6',
    elements: [],
  }
  return {
    id,
    position: 0,
    type: 'slide',
    title: id,
    knowledge_points: [],
    status: 'done',
    content,
    steps,
    questions: [],
  }
}

function seedDetail(store: ReturnType<typeof useLearnStore>, sections: SectionLearn[]) {
  store.detail = {
    course: { id: 'c1', title: '课程' },
    progress: 'in_progress',
    sections,
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('learn store 白板笔画回放', () => {
  it('笔画跨环节累积：切换到下一环节不清空', () => {
    const store = useLearnStore()
    seedDetail(store, [
      slideSection('s1', [
        { text: 'a', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } }] },
        { text: 'b' },
      ]),
      slideSection('s2', [
        { text: 'c', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[2, 2]] } }] },
      ]),
    ])
    // 处于第一节第 0 步：只有 s1 第 0 步的笔画
    expect(store.whiteboardStrokes).toHaveLength(1)
    // 切到下一环节（第 2 节第 0 步）：s1 的笔画累积保留
    store.goTo(1)
    expect(store.currentIndex).toBe(1)
    expect(store.whiteboardStrokes).toHaveLength(2)
    // 回到上一环节：重放收缩到该位置之前的状态（自动回退，无重复作画）
    store.goTo(0)
    expect(store.whiteboardStrokes).toHaveLength(1)
  })

  it('clearBoard 清空全部已累积笔画，之后重新累积', () => {
    const store = useLearnStore()
    seedDetail(store, [
      slideSection('s1', [
        { text: 'a', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } }] },
        { text: 'b', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } }] },
      ]),
      slideSection('s2', [
        { text: 'c', actions: [{ type: 'clearBoard' }] },
        { text: 'd', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } }] },
      ]),
    ])
    store.goTo(1)
    expect(store.stepIndex).toBe(0)
    // 当前步骤（s2 第 0 步）的 clearBoard 已生效 → s1 的 2 笔被清空
    expect(store.whiteboardStrokes).toHaveLength(0)
    store.nextStep()
    expect(store.whiteboardStrokes).toHaveLength(1)
  })

  it('只取当前位置之前的笔画，超前的步骤不提前出现', () => {
    const store = useLearnStore()
    seedDetail(store, [
      slideSection('s1', [
        { text: 'a', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } }] },
        { text: 'b', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } }] },
        { text: 'c', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } }] },
      ]),
    ])
    expect(store.whiteboardStrokes).toHaveLength(1)
    store.nextStep()
    expect(store.whiteboardStrokes).toHaveLength(2)
    store.nextStep()
    expect(store.whiteboardStrokes).toHaveLength(3)
    store.prevStep()
    expect(store.whiteboardStrokes).toHaveLength(2)
  })

  it('laser 与 draw 动作类型互不干扰', () => {
    const store = useLearnStore()
    seedDetail(store, [
      slideSection('s1', [
        {
          text: 'a',
          actions: [
            { type: 'laser', targetElementId: 'e1' },
            { type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } },
          ],
        },
      ]),
    ])
    expect(store.whiteboardStrokes).toHaveLength(1)
  })
})

describe('learn store 视图派生', () => {
  // 视图为纯用户选择的会话级状态（AI 动作不控制视图）
  it('默认遮罩书写；三态手动切换且跨步骤/跨环节保持', () => {
    const store = useLearnStore()
    seedDetail(store, [
      slideSection('s1', [{ text: 'a', actions: [{ type: 'laser', targetElementId: 'e1' }] }]),
      slideSection('s2', [{ text: 'b' }]),
    ])
    // 默认遮罩书写
    expect(store.manualView).toBe('overlay')
    // 三态切换
    store.setSlideView('board')
    expect(store.manualView).toBe('board')
    store.setSlideView('slide')
    expect(store.manualView).toBe('slide')
    // 跨步骤保持
    store.nextStep()
    expect(store.manualView).toBe('slide')
    // 跨环节保持
    store.goTo(1)
    expect(store.manualView).toBe('slide')
  })

  it('视图切换不影响笔画序列（回放与视图解耦）', () => {
    const store = useLearnStore()
    seedDetail(store, [
      slideSection('s1', [
        { text: 'a', actions: [{ type: 'draw', drawing: { kind: 'pen', points: [[1, 1]] } }] },
      ]),
    ])
    const strokesInBoard = (() => {
      store.setSlideView('board')
      const n = store.whiteboardStrokes.length
      store.setSlideView('overlay')
      return n === store.whiteboardStrokes.length
    })()
    expect(strokesInBoard).toBe(true)
  })

  it('当前步骤的 laser 动作作为瞬时状态暴露', () => {
    const store = useLearnStore()
    seedDetail(store, [
      slideSection('s1', [
        { text: 'a', actions: [{ type: 'laser', targetElementId: 'e1' }] },
        { text: 'b', actions: [{ type: 'laser', x: 640, y: 300 }] },
      ]),
    ])
    expect(store.laserAction?.targetElementId).toBe('e1')
    store.nextStep()
    expect(store.laserAction?.x).toBe(640)
    expect(store.laserAction?.y).toBe(300)
  })
})
