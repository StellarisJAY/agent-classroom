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
