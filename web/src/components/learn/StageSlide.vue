<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { renderToString } from 'katex'

import type {
  SlideAction,
  SlideContent,
  SlideElement,
  SlideFormulaElement,
  SlideImageElement,
  SlideListElement,
  SlideMermaidElement,
  SlideShapeElement,
  SlideTextElement,
} from '@/api/learn'
import { useLearnStore } from '@/stores/learn'

const store = useLearnStore()

const content = computed<SlideContent | null>(() => store.slideContent)

// ---- 画布等比缩放（object-fit: contain 的 JS 等价）----
const regionRef = ref<HTMLElement | null>(null)
const canvasRef = ref<HTMLElement | null>(null)
const scale = ref(1)
let ro: ResizeObserver | null = null

function fit() {
  const region = regionRef.value
  const c = content.value
  if (!region || !c) return
  const rw = region.clientWidth
  const rh = region.clientHeight
  if (rw <= 0 || rh <= 0) return
  scale.value = Math.min(rw / c.width, rh / c.height)
}

onMounted(() => {
  fit()
  if (regionRef.value) {
    ro = new ResizeObserver(() => {
      fit()
      nextTick(measureOverlays)
    })
    ro.observe(regionRef.value)
  }
  nextTick(measureOverlays)
})

onBeforeUnmount(() => ro?.disconnect())

const cssSize = computed(() => {
  const c = content.value
  const s = scale.value
  return c ? { width: `${c.width * s}px`, height: `${c.height * s}px` } : {}
})

// ---- 元素定位 / 样式（坐标换算为缩放后 px）----
const px = (v?: number) => `${(v ?? 0) * scale.value}px`

function elementStyle(el: SlideElement) {
  return { left: px(el.x), top: px(el.y) }
}

function textStyle(el: SlideTextElement) {
  const st = el.style ?? {}
  return {
    left: px(el.x),
    top: px(el.y),
    width: px(el.width),
    fontSize: px(st.fontSize ?? 28),
    fontWeight: st.bold ? 600 : undefined,
    textAlign: st.align ?? 'left',
    color: st.color ?? undefined,
  }
}

function listStyle(el: SlideListElement) {
  return {
    left: px(el.x),
    top: px(el.y),
    width: px(el.width),
    fontSize: px(el.fontSize ?? 26),
  }
}

function formulaHtml(el: SlideFormulaElement): string {
  try {
    return renderToString(el.content, { throwOnError: false })
  } catch {
    return ''
  }
}

function formulaStyle(el: SlideFormulaElement) {
  return { left: px(el.x), top: px(el.y), fontSize: px(el.fontSize ?? 32) }
}

function shapeBox(el: SlideShapeElement) {
  return {
    width: px(el.width),
    height: px(el.height),
    background: el.fill ?? 'transparent',
    borderColor: el.stroke ?? content.value?.accent ?? '#14b8a6',
  }
}

function shapeKind(el: SlideShapeElement): string {
  if (el.shape === 'circle') return 'is-circle'
  if (el.shape === 'line') return 'is-line'
  if (el.shape === 'arrow') return 'is-arrow'
  return ''
}

function imageStyle(el: SlideImageElement) {
  return { left: px(el.x), top: px(el.y), width: px(el.width), height: px(el.height) }
}

function isText(e: SlideElement): e is SlideTextElement {
  return e.type === 'text'
}
function isFormula(e: SlideElement): e is SlideFormulaElement {
  return e.type === 'formula'
}
function isShape(e: SlideElement): e is SlideShapeElement {
  return e.type === 'shape'
}
function isList(e: SlideElement): e is SlideListElement {
  return e.type === 'list'
}
function isImage(e: SlideElement): e is SlideImageElement {
  return e.type === 'image'
}
function isMermaid(e: SlideElement): e is SlideMermaidElement {
  return e.type === 'mermaid'
}

// ---- mermaid 流程图渲染（动态导入，源码 → SVG 缓存） ----
interface MermaidRendered {
  svg: string
  viewBox: { w: number; h: number } | null
  bindFunctions?: (el: HTMLElement) => void
}

const mermaidOutputs = ref(new Map<string, MermaidRendered | null>())
let mermaidSeq = 0

async function renderMermaid(el: SlideMermaidElement) {
  const key = `${el.id}@${el.content}`
  if (mermaidOutputs.value.has(key)) return
  mermaidOutputs.value.set(key, null)
  try {
    const mermaid = (await import('mermaid')).default
    const { initialize } = mermaid
    initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'base',
      themeVariables: { fontFamily: 'inherit' },
      flowchart: { useMaxWidth: false },
    })
    const { svg, bindFunctions } = await mermaid.render(`stage-mermaid-${++mermaidSeq}`, el.content)
    mermaidOutputs.value.set(key, { svg, viewBox: parseSvgSize(svg), bindFunctions })
    await nextTick(measureOverlays)
  } catch {
    mermaidOutputs.value.set(key, null)
  }
}

/** 从 svg 字符串解析内在尺寸：优先 viewBox，回退 width/height 属性。 */
function parseSvgSize(svg: string): { w: number; h: number } | null {
  const vb = svg.match(/viewBox="[^"]*\s([\d.]+)\s([\d.]+)"/)
  if (vb) {
    const w = Number(vb[1])
    const h = Number(vb[2])
    if (w > 0 && h > 0) return { w, h }
  }
  const wm = svg.match(/width="([\d.]+)/)
  const hm = svg.match(/height="([\d.]+)/)
  if (wm && hm) {
    const w = Number(wm[1])
    const h = Number(hm[1])
    if (w > 0 && h > 0) return { w, h }
  }
  return null
}

function mermaidOutputOf(el: SlideMermaidElement): MermaidRendered | null {
  return mermaidOutputs.value.get(`${el.id}@${el.content}`) ?? null
}

/** mermaid 距画布底边的最小边距（画布坐标系 px） */
const MERMAID_BOTTOM_MARGIN = 24

/** 计算流程图显示尺寸（画布坐标系）：等比缩放到「不超 el.width、不超画布底边」，小幅图放大到宽度上限。 */
function mermaidSize(el: SlideMermaidElement, c: SlideContent): { w: number; h?: number } {
  const vb = mermaidOutputOf(el)?.viewBox
  const availW = Math.max(el.width, 0)
  const availH = c.height - el.y - MERMAID_BOTTOM_MARGIN
  if (!vb || availW <= 0 || availH <= 0) return { w: availW }
  const s = Math.min(availW / vb.w, availH / vb.h)
  return { w: vb.w * s, h: vb.h * s }
}

function mermaidBox(el: SlideMermaidElement) {
  const c = content.value
  const size = c ? mermaidSize(el, c) : { w: el.width }
  return {
    left: px(el.x),
    top: px(el.y),
    width: px(size.w),
    height: size.h ? px(size.h) : undefined,
    fontSize: px(el.fontSize ?? 16),
  }
}

// ---- 步骤动作叠加层（underline / highlight / box，多动作）----
interface OverlayRect {
  key: string
  type: SlideAction['type']
  left: number
  top: number
  width: number
  height: number
}

/** underline 下划线厚度（px，画布坐标系） */
const UNDERLINE_THICKNESS = 2

const elementRefs = new Map<string, HTMLElement>()
const overlays = ref<OverlayRect[]>([])

function setRef(id: string, node: unknown) {
  if (node instanceof HTMLElement) elementRefs.set(id, node)
  else elementRefs.delete(id)
}

function measureOverlays() {
  const canvasEl = canvasRef.value
  const step = store.currentStep
  overlays.value = []
  if (!canvasEl || !step?.actions?.length) return
  const base = canvasEl.getBoundingClientRect()
  const list: OverlayRect[] = []
  for (const action of step.actions) {
    const el = elementRefs.get(action.targetElementId)
    if (!el) continue
    const r = el.getBoundingClientRect()
    if (action.type === 'underline') {
      // underline：只在目标元素底边画一条细横线，不覆盖元素本身
      const thickness = UNDERLINE_THICKNESS
      list.push({
        key: `${action.type}-${action.targetElementId}`,
        type: action.type,
        left: r.left - base.left,
        top: r.bottom - base.top - thickness,
        width: r.width,
        height: thickness,
      })
    } else {
      list.push({
        key: `${action.type}-${action.targetElementId}`,
        type: action.type,
        left: r.left - base.left,
        top: r.top - base.top,
        width: r.width,
        height: r.height,
      })
    }
  }
  overlays.value = list
}

// 解析到 mermaid 元素后触发渲染。
watch(
  content,
  (c) => {
    if (!c) return
    for (const el of c.elements) {
      if (isMermaid(el)) void renderMermaid(el)
    }
  },
  { deep: true },
)

watch([() => store.stepIndex, () => store.currentIndex], async () => {
  await nextTick()
  fit()
  await nextTick()
  measureOverlays()
})
</script>

<template>
  <div class="stage-slide">
    <div ref="regionRef" class="stage-slide__region">
      <div
        v-if="content"
        ref="canvasRef"
        class="stage-slide__canvas"
        :style="{ ...cssSize, background: content.background }"
      >
        <template v-for="el in content.elements" :key="el.id">
          <div
            v-if="isText(el)"
            :ref="(n) => setRef(el.id, n)"
            class="stage-el stage-el--text"
            :style="textStyle(el)"
          >
            {{ el.content }}
          </div>

          <div
            v-else-if="isFormula(el)"
            :ref="(n) => setRef(el.id, n)"
            class="stage-el stage-el--formula"
            :style="formulaStyle(el)"
            v-html="formulaHtml(el)"
          />

          <div
            v-else-if="isShape(el)"
            :ref="(n) => setRef(el.id, n)"
            class="stage-el stage-el--shape"
            :class="shapeKind(el)"
            :style="{ ...elementStyle(el), ...shapeBox(el) }"
          >
            <span v-if="el.label" class="stage-el--shape-label">{{ el.label }}</span>
          </div>

          <div
            v-else-if="isList(el)"
            :ref="(n) => setRef(el.id, n)"
            class="stage-el stage-el--list"
            :style="listStyle(el)"
          >
            <ol v-if="el.ordered" class="stage-el--list-ol">
              <li v-for="(it, i) in el.items" :key="i">{{ it }}</li>
            </ol>
            <ul v-else class="stage-el--list-ul">
              <li v-for="(it, i) in el.items" :key="i">{{ it }}</li>
            </ul>
          </div>

          <img
            v-else-if="isImage(el)"
            :ref="(n) => setRef(el.id, n)"
            class="stage-el stage-el--image"
            :style="imageStyle(el)"
            :src="el.src"
            :alt="el.prompt ?? ''"
          />

          <div
            v-else-if="isMermaid(el) && mermaidOutputOf(el)"
            :ref="(n) => setRef(el.id, n)"
            class="stage-el stage-el--mermaid"
            :style="mermaidBox(el)"
          >
            <div class="stage-el--mermaid-svg" v-html="mermaidOutputOf(el)?.svg" />
          </div>
        </template>

        <div
          v-for="o in overlays"
          :key="o.key"
          class="stage-overlay"
          :class="`is-${o.type}`"
          :style="{
            left: `${o.left}px`,
            top: `${o.top}px`,
            width: `${o.width}px`,
            height: `${o.height}px`,
            borderColor: content.accent,
            background:
              o.type === 'highlight'
                ? `${content.accent}33`
                : o.type === 'underline'
                  ? content.accent
                  : undefined,
          }"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.stage-slide {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  min-height: 0;
}

.stage-slide__region {
  flex: 1;
  min-height: 0;
  display: grid;
  place-items: center;
  overflow: hidden;
  padding: 8px;
}

.stage-slide__canvas {
  position: relative;
  overflow: hidden;
  box-shadow: 0 1px 6px rgba(15, 23, 42, 0.08);
  border-radius: 4px;
}

.stage-el {
  position: absolute;
  line-height: 1.4;
}

.stage-el--text {
  white-space: pre-wrap;
  overflow: hidden;
}

.stage-el--formula {
  color: #111827;
  overflow: visible;
}
.stage-el--formula :deep(.katex) {
  font-size: 1em;
}

.stage-el--shape {
  display: flex;
  align-items: center;
  justify-content: center;
  border-width: 2px;
  border-style: solid;
  color: #0f172a;
}
.stage-el--shape.is-circle {
  border-radius: 50%;
}
.stage-el--shape.is-line {
  border: none;
  background: currentColor;
}
.stage-el--shape.is-arrow {
  border: none;
  background: currentColor;
  height: 3px !important;
}
.stage-el--shape-label {
  font-weight: 500;
}

.stage-el--list {
  color: #334155;
}
.stage-el--image {
  object-fit: contain;
  border-radius: 4px;
}

.stage-el--mermaid {
  display: flex;
  align-items: center;
}
.stage-el--mermaid-svg {
  width: 100%;
}
.stage-el--mermaid-svg :deep(svg) {
  display: block;
  margin: 0 auto;
}
.stage-el--mermaid-svg :deep(svg[viewBox]) {
  width: 100%;
  height: auto;
  max-width: none;
}
.stage-el--list-ol,
.stage-el--list-ul {
  margin: 0;
  padding-left: 1.2em;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stage-overlay {
  position: absolute;
  pointer-events: none;
  box-sizing: border-box;
}
.stage-overlay.is-box {
  border: 2px solid;
  border-radius: 3px;
}
.stage-overlay.is-highlight {
  border-radius: 3px;
}
.stage-overlay.is-underline {
  border: none;
  border-radius: 1px;
}
</style>
