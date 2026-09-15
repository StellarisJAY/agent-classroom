import { describe, expect, it } from 'vitest'

import type { SlideDrawing, SlideStroke } from '@/api/learn'
import {
  BOARD_H,
  BOARD_W,
  collectReplayStrokes,
  renormalizeDrawing,
  sectionCanvasSize,
  strokeOrder,
} from '@/stores/learnStrokes'

function pen(pts: [number, number][]): SlideDrawing {
  return { kind: 'pen', points: pts, color: '#123456' }
}

interface FakeSection {
  id: string
  type: string
  status: string
  steps?: { actions?: { type: string; drawing?: SlideDrawing }[] }[]
  content?: { width: number; height: number } | null
  questions?: unknown[]
}

describe('learnStrokes', () => {
  it('strokeOrder 按 section/step 编码且跨环节单调', () => {
    expect(strokeOrder(0, 999)).toBe(999)
    expect(strokeOrder(1, 0)).toBeGreaterThan(strokeOrder(0, 999))
  })

  it('renormalizeDrawing 缩放点集与包围盒；同尺寸原样返回', () => {
    const d: SlideDrawing = { kind: 'rect', x: 640, y: 360, width: 100, height: 50 }
    const normalized = renormalizeDrawing(d, 2, 2)
    expect(normalized.x).toBe(1280)
    expect(normalized.width).toBe(200)
    expect(renormalizeDrawing(d, 1, 1)).toBe(d)
  })

  it('collectReplayStrokes：跨环节累积、回退剪裁、clearBoard 清空重算', () => {
    const sections: FakeSection[] = [
      {
        id: 's1',
        type: 'slide',
        status: 'done',
        content: { width: 640, height: 720 },
        steps: [
          { actions: [{ type: 'draw', drawing: pen([[10, 10]]) }, { type: 'clearBoard' }] },
          { actions: [{ type: 'draw', drawing: pen([[320, 360]]) }] },
        ],
      },
      {
        id: 's2',
        type: 'slide',
        status: 'done',
        content: { width: 1280, height: 720 },
        steps: [{ actions: [{ type: 'draw', drawing: pen([[100, 100]]) }] }],
      },
      {
        id: 's3',
        type: 'quiz',
        status: 'done',
        questions: [],
        steps: [],
      },
    ]
    const asReplay = sections as unknown as Parameters<typeof collectReplayStrokes>[0]

    // 位于第 1 环节第 0 步：clearBoard 已把第一步笔画清空
    expect(collectReplayStrokes(asReplay, 0, 0)).toHaveLength(0)
    // 第 1 环节第 1 步：clearBoard 之后的一笔；坐标从 640 宽画布放大到 1280
    const at0_1 = collectReplayStrokes(asReplay, 0, 1)
    expect(at0_1).toHaveLength(1)
    expect(at0_1[0]!.drawing.points![0]).toEqual([640, 360])
    // 第 2 环节追加一笔
    const at1_0 = collectReplayStrokes(asReplay, 1, 0)
    expect(at1_0).toHaveLength(2)
    expect(at1_0[0]!.order).toBe(strokeOrder(0, 1))
    expect(at1_0[1]!.order).toBe(strokeOrder(1, 0))
  })

  it('sectionCanvasSize 缺省 1280×720', () => {
    expect(sectionCanvasSize({ type: 'slide', content: null } as never)).toEqual({
      w: BOARD_W,
      h: BOARD_H,
    })
  })
})

describe('白板重放的类型冒烟（SlideStroke 形态）', () => {
  it('order + drawing 字段', () => {
    const s: SlideStroke = { order: 1000, drawing: pen([[0, 0]]) }
    expect(s.order).toBe(1000)
  })
})
