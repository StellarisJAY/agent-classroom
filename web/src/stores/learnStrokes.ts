import type {
  SectionLearn,
  SlideContent,
  SlideDrawing,
  SlideStroke,
} from '@/api/learn'

/**
 * 白板笔画派生纯函数：从课程环节数据推导"截至某个位置应呈现的笔画序列"。
 * 从 learn store 抽出为无状态纯函数，便于单测与讨论模式叠加层复用归一化逻辑。
 */

/** 白板逻辑坐标系（跨环节统一对齐的目标画布尺寸）。 */
export const BOARD_W = 1280
export const BOARD_H = 720

/** 单个环节内笔画顺序的空间：order = sectionIndex * SECTION_STEP + stepIndex。 */
export const SECTION_STEP = 1000

/** sectionIndex + stepIndex 编码为全局回放顺序（同一环节内按步骤递增）。 */
export function strokeOrder(sectionIndex: number, stepIndex: number): number {
  return sectionIndex * SECTION_STEP + stepIndex
}

/** 把产出环节画布尺寸下的坐标归一化到白板逻辑坐标系。 */
export function renormalizeDrawing(
  d: SlideDrawing,
  sx: number,
  sy: number,
): SlideDrawing {
  if (sx === 1 && sy === 1) return d
  const scalePoints = (pts?: [number, number][]) =>
    pts?.map(([px, py]) => [px * sx, py * sy] as [number, number])
  return {
    ...d,
    points: scalePoints(d.points),
    x: d.x !== undefined ? d.x * sx : undefined,
    y: d.y !== undefined ? d.y * sy : undefined,
    width: d.width !== undefined ? d.width * sx : undefined,
    height: d.height !== undefined ? d.height * sy : undefined,
  }
}

/** 读取环节内容画布尺寸（缺省 1280×720）。 */
export function sectionCanvasSize(section: SectionLearn): { w: number; h: number } {
  const c = section.content as SlideContent | null
  return {
    w: c?.width && c.width > 0 ? c.width : BOARD_W,
    h: c?.height && c.height > 0 ? c.height : BOARD_H,
  }
}

interface ReplaySection {
  type: string
  status: string
  steps?: unknown
  content?: unknown
}

/**
 * 全局笔画重放：白板笔画跨环节累积，行进到 (currentIndex, stepIndex) 时
 * 取所有"位置 ≤ 当前"的 draw 按序重放，遇 clearBoard 清空重算。
 * 坐标按产出环节画布尺寸归一化（sx/sy 相对逻辑坐标系）。
 */
export function collectReplayStrokes(
  sections: ReplaySection[],
  currentIndex: number,
  stepIndex: number,
): SlideStroke[] {
  const out: SlideStroke[] = []
  for (let si = 0; si < sections.length; si++) {
    const s = sections[si]
    if (!s) continue
    if (si > currentIndex) break
    if (s.type !== 'slide' || s.status !== 'done') continue
    const steps = Array.isArray(s.steps) ? s.steps : []
    const limit = si === currentIndex ? stepIndex : steps.length - 1
    const { w: cw, h: ch } = sectionCanvasSize(s as SectionLearn)
    const sx = BOARD_W / cw
    const sy = BOARD_H / ch
    for (let j = 0; j <= limit; j++) {
      const actions = (steps[j] as { actions?: unknown[] } | undefined)?.actions ?? []
      for (const a of actions as { type: string; drawing?: SlideDrawing }[]) {
        if (a.type === 'clearBoard') {
          out.length = 0
        } else if (a.type === 'draw' && a.drawing) {
          out.push({
            order: strokeOrder(si, j),
            drawing: renormalizeDrawing(a.drawing, sx, sy),
          })
        }
      }
    }
  }
  return out
}
