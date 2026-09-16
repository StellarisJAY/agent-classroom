<script setup lang="ts">
// WhiteboardLayer：常驻于 slide 画布上的电子白板遮罩层（同一坐标系）。
// 三视图：slide（隐藏）/ overlay（透明遮罩，笔画叠在幻灯片上）/ board（不透明白板）。
// 笔画为声明式回放（由 learn store 派生），每次变化整画布重绘 → 步进回退自然收缩，
// 不会重复作画；新追加的笔画逐笔浮现（~180ms/笔），回退时静止呈现。
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import type { SlideDrawSize, SlideDrawing, SlideStroke, SlideView } from '@/api/learn'

const props = withDefaults(
  defineProps<{
    strokes: SlideStroke[]
    view: SlideView
    accent: string
    /** 画布逻辑尺寸（坐标系统一为 1280×720，由调用方归一化） */
    canvasWidth: number
    canvasHeight: number
    /** 画布的展示缩放比（逻辑坐标 → 屏幕px） */
    scale: number
    /**
     * 讨论模式叠加笔画：独立于正式回放流（不参与逐笔浮现动画与 lastBaseCount
     * 基准）、不写入 whiteboardStrokes 回放序列；讨论结束时整层清空。
     * 为空/未传时不渲染叠加画布。
     */
    overlayStrokes?: SlideStroke[]
  }>(),
  { overlayStrokes: () => [] },
)

/** 各笔画粗细的绘制线宽（逻辑坐标 px，随 scale 放大） */
const SIZES: Record<SlideDrawSize, number> = { thin: 2, medium: 4, thick: 6 }
/** 逐笔浮现的间隔 */
const STROKE_DELAY_MS = 180

const canvasRef = ref<HTMLCanvasElement | null>(null)
const overlayCanvasRef = ref<HTMLCanvasElement | null>(null)

/** 当前实际绘制的笔画数（重放全量与逐笔动画 Visibility 合成后的切片） */
const visibleCount = ref(0)

let animTimer: ReturnType<typeof setTimeout> | null = null
let animCancel = 0

function stopAnim() {
  animCancel++
  if (animTimer) {
    clearTimeout(animTimer)
    animTimer = null
  }
}

const displaySize = computed(() => ({
  w: props.canvasWidth * props.scale,
  h: props.canvasHeight * props.scale,
}))

const toPx = (v: number) => v * props.scale

// ---- 绘制 ----

function strokeStyle(d: SlideDrawing) {
  return {
    color: d.color || props.accent,
    lineWidth: SIZES[d.size ?? 'medium'] ?? 4,
  }
}

/** 全量重绘 visibleCount 条（从队首到 visibleCount，声明式回放） */
function redraw() {
  stopAnim()
  visibleCount.value = props.view === 'slide' ? 0 : props.strokes.length
  paint(0, visibleCount.value)
  animateNewlyAdded()
}

/** 记录上次静态重绘时的笔画数，重放序列新增时对新增部分逐笔浮现 */
let lastBaseCount = 0

function animateNewlyAdded() {
  const total = visibleCount.value
  const base = lastBaseCount
  lastBaseCount = total
  // 首次载入（无基准）静态全量呈现：整段回放动画会拖沓
  if (total <= base || base === 0 || props.view === 'slide') return

  const seq = ++animCancel
  let shown = base
  const stepShow = () => {
    if (seq !== animCancel) return
    shown += 1
    paint(Math.max(0, shown - 1), shown)
    if (shown < total) {
      animTimer = setTimeout(stepShow, STROKE_DELAY_MS)
    }
  }
  if (animTimer) clearTimeout(animTimer)
  animTimer = setTimeout(stepShow, 0)
}

function paint(from: number, to: number) {
  const cv = canvasRef.value
  if (!cv) return
  const ctx = cv.getContext('2d')
  if (!ctx) return

  const dp = window.devicePixelRatio || 1
  const w = displaySize.value.w
  const h = displaySize.value.h
  // 画布不清 box-shadow 等周围区域：清理到逻辑画布全域
  if (cv.width !== Math.round(w * dp) || cv.height !== Math.round(h * dp)) {
    cv.width = Math.round(w * dp)
    cv.height = Math.round(h * dp)
  }
  ctx.setTransform(dp, 0, 0, dp, 0, 0)
  if (from === 0) ctx.clearRect(0, 0, w, h)
  if (props.view !== 'slide' && from === 0) {
    // board 视图的白色底（等价"切换到白板"），overlay 为纯透明
    if (props.view === 'board') {
      ctx.fillStyle = '#ffffff'
      ctx.fillRect(0, 0, w, h)
    }
  }

  const strokes = props.strokes
  for (let i = from; i < Math.min(to, strokes.length); i++) {
    const s = strokes[i]
    if (!s) continue
    drawStroke(ctx, s.drawing)
  }
}

function drawStroke(ctx: CanvasRenderingContext2D, d: SlideDrawing) {
  const { color, lineWidth } = strokeStyle(d)
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.strokeStyle = color
  ctx.lineWidth = toPx(lineWidth)
  ctx.fillStyle = color

  const pts = d.points
  if (d.kind === 'pen' && pts && pts.length > 0) {
    const first = pts[0]
    let prev: [number, number]
    if (!first) return
    prev = first
    ctx.beginPath()
    ctx.moveTo(toPx(prev[0]), toPx(prev[1]))
    for (let i = 1; i < pts.length; i++) {
      const cur = pts[i]
      if (!cur) break
      // 中点二次平滑，让 pen 更像真人笔迹
      const mx = toPx((prev[0] + cur[0]) / 2)
      const my = toPx((prev[1] + cur[1]) / 2)
      ctx.quadraticCurveTo(toPx(prev[0]), toPx(prev[1]), mx, my)
      prev = cur
    }
    ctx.lineTo(toPx(prev[0]), toPx(prev[1]))
    ctx.stroke()
    return
  }
  if ((d.kind === 'line' || d.kind === 'arrow') && pts && pts.length >= 2) {
    const a = pts[0]
    const b = pts[1]
    if (!a || !b) return
    ctx.beginPath()
    ctx.moveTo(toPx(a[0]), toPx(a[1]))
    ctx.lineTo(toPx(b[0]), toPx(b[1]))
    ctx.stroke()
    if (d.kind === 'arrow') arrowHead(ctx, a, b, color)
    return
  }
  if (d.kind === 'rect' || d.kind === 'circle') {
    const x = toPx(d.x ?? 0)
    const y = toPx(d.y ?? 0)
    const w = toPx(d.width ?? 0)
    const h = toPx(d.height ?? 0)
    if (w <= 0 || h <= 0) return
    ctx.beginPath()
    if (d.kind === 'rect') ctx.rect(x, y, w, h)
    else ctx.ellipse(x + w / 2, y + h / 2, Math.abs(w / 2), Math.abs(h / 2), 0, 0, Math.PI * 2)
    ctx.stroke()
    return
  }
  if (d.kind === 'text' && d.content) {
    const fontSize = (d.fontSize && d.fontSize > 0 ? d.fontSize : 24)
    ctx.font = `${toPx(fontSize)}px ui-sans-serif, system-ui, sans-serif`
    ctx.textBaseline = 'top'
    ctx.fillText(d.content, toPx(d.x ?? 0), toPx(d.y ?? 0))
  }
}

function arrowHead(
  ctx: CanvasRenderingContext2D,
  a: [number, number],
  b: [number, number],
  color: string,
) {
  // 箭头尖端的短三角
  const fmt = toPx(1)
  const ang = Math.atan2(b[1] - a[1], b[0] - a[0])
  const size = Math.max(fmt * 10, SIZES.medium * props.scale * 2)
  ctx.save()
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.moveTo(toPx(b[0]), toPx(b[1]))
  ctx.lineTo(
    toPx(b[0]) - size * Math.cos(ang - Math.PI / 7),
    toPx(b[1]) - size * Math.sin(ang - Math.PI / 7),
  )
  ctx.lineTo(
    toPx(b[0]) - size * Math.cos(ang + Math.PI / 7),
    toPx(b[1]) - size * Math.sin(ang + Math.PI / 7),
  )
  ctx.closePath()
  ctx.fill()
  ctx.restore()
}

// ---- 状态变化 → 重放重绘 ----

// strokes 数组是 store 派生（每步重建），watch 引用变化整体重放
watch(
  () => props.strokes,
  () => redraw(),
)
// 讨论模式叠加层：声明式整层重绘（无动画、无基准 hack），变化即全量呈现；
// post 保证叠加画布 v-if 挂载完成后再绘制
watch(
  () => props.overlayStrokes,
  () => paintOverlay(),
  { flush: 'post' },
)
watch(
  () => props.view,
  (v) => {
    stopAnim()
    lastBaseCount = 0
    visibleCount.value = v === 'slide' ? 0 : props.strokes.length
    paint(0, visibleCount.value)
    paintOverlay()
  },
)
// 尺寸/缩放变化重绘（切环节、窗口 resize 时）
watch(
  () => [props.canvasWidth, props.canvasHeight, props.scale],
  () => {
    paint(0, visibleCount.value)
    paintOverlay()
  },
)

/** 叠加层整层重绘：只画 overlayStrokes，不叠任何背景与动画。 */
function paintOverlay() {
  const cv = overlayCanvasRef.value
  if (!cv) return
  const ctx = cv.getContext('2d')
  if (!ctx) return
  const dp = window.devicePixelRatio || 1
  const w = displaySize.value.w
  const h = displaySize.value.h
  if (cv.width !== Math.round(w * dp) || cv.height !== Math.round(h * dp)) {
    cv.width = Math.round(w * dp)
    cv.height = Math.round(h * dp)
  }
  ctx.setTransform(dp, 0, 0, dp, 0, 0)
  ctx.clearRect(0, 0, w, h)
  if (props.view === 'slide') return
  for (const s of props.overlayStrokes) {
    drawStroke(ctx, s.drawing)
  }
}

onBeforeUnmount(stopAnim)

defineExpose({
  repaint: () => {
    paint(0, visibleCount.value)
    paintOverlay()
  },
})

</script>
<template>
  <div
    class="whiteboard-layer"
    :class="`is-${view}`"
    :style="{ width: `${displaySize.w}px`, height: `${displaySize.h}px` }"
  >
    <canvas ref="canvasRef" class="whiteboard-layer__canvas" />
    <canvas
      v-if="overlayStrokes.length"
      ref="overlayCanvasRef"
      class="whiteboard-layer__canvas whiteboard-layer__overlay"
      aria-label="讨论模式手绘叠加层"
    />
  </div>
</template>

<style scoped>
.whiteboard-layer {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}
.whiteboard-layer.is-slide {
  display: none;
}
.whiteboard-layer__canvas {
  display: block;
  /* 显示尺寸恒等于容器（即 slide 逻辑尺寸 × scale），
     位图属性按 devicePixelRatio 的超采样在 CSS 缩放中还原，保证高 DPI 下与 slide 对齐 */
  width: 100%;
  height: 100%;
}
.whiteboard-layer__overlay {
  position: absolute;
  inset: 0;
}
</style>
