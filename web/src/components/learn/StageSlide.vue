<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { renderToString } from 'katex'

import type { ECharts } from 'echarts/core'
import type { Chart, FunctionPlotOptions, FunctionPlotDatum } from 'function-plot'
import type {
  SlideAction,
  SlideChartElement,
  SlideContent,
  SlideElement,
  SlideFormulaElement,
  SlideFunctionPlotElement,
  SlideImageElement,
  SlideListElement,
  SlideMermaidElement,
  SlideShapeElement,
  SlideTextElement,
} from '@/api/learn'
import { useLearnStore } from '@/stores/learn'
import { useDiscussionStore } from '@/stores/discussion'
import WhiteboardLayer from '@/components/learn/WhiteboardLayer.vue'

const store = useLearnStore()
const discussion = useDiscussionStore()

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
function isChart(e: SlideElement): e is SlideChartElement {
  return e.type === 'chart'
}
function isFunctionPlot(e: SlideElement): e is SlideFunctionPlotElement {
  return e.type === 'functionPlot'
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

// ---- chart 统计图渲染（动态按需导入 echarts，SVG 渲染，实例缓存） ----
interface ChartEntry {
  /** 元素数据签名（id + 数据），用于判断缓存是否失效 */
  key: string
  inst: ECharts | null
}

async function loadEcharts() {
  const core = await import('echarts/core')
  const { BarChart, LineChart, PieChart } = await import('echarts/charts')
  const { GridComponent, TitleComponent, LegendComponent } = await import('echarts/components')
  const { SVGRenderer } = await import('echarts/renderers')
  core.use([BarChart, LineChart, PieChart, GridComponent, TitleComponent, LegendComponent, SVGRenderer])
  return { init: core.init }
}

/** 元素 id → 图表实例（元素移除/数据变化时 dispose） */
const chartEntries = new Map<string, ChartEntry>()
const chartRefs = new Map<string, HTMLElement>()
let echartsLoader: Promise<Awaited<ReturnType<typeof loadEcharts>>> | null = null

function loadEchartsOnce() {
  echartsLoader ??= loadEcharts()
  return echartsLoader
}

function setChartRef(id: string, node: unknown) {
  if (node instanceof HTMLElement) chartRefs.set(id, node)
  else chartRefs.delete(id)
}

async function renderChart(el: SlideChartElement, accent: string) {
  const key = `${el.id}@${JSON.stringify({ ...el, id: '', x: 0, y: 0 })}`
  const entry = chartEntries.get(el.id)
  if (entry && entry.key === key) return
  entry?.inst?.dispose()
  chartEntries.set(el.id, { key, inst: null })
  try {
    await nextTick()
    const dom = chartRefs.get(el.id)
    if (!dom) return
    const { init } = await loadEchartsOnce()
    const option = buildChartOption(el, accent)
    const inst = init(dom, null, { renderer: 'svg', width: el.width, height: el.height })
    inst.setOption(option)
    chartEntries.set(el.id, { key, inst })
    await nextTick(measureOverlays)
  } catch {
    // 渲染失败：保持 inst 为 null，元素不显示，不影响其余内容（与 mermaid 一致）
  }
}

/** accent 派生色板：强调色打头，后续系列用固定的和谐补充色 */
function chartPalette(accent: string): string[] {
  return [
    accent,
    '#6366f1',
    '#f59e0b',
    '#ef4444',
    '#10b981',
    '#3b82f6',
    '#8b5cf6',
    '#f97316',
  ]
}

/** 由封闭的宽表数据确定性构建 echarts option（硬编码全部样式，模型只给数据） */
function buildChartOption(el: SlideChartElement, accent: string): Record<string, unknown> {
  const palette = chartPalette(accent)
  const title = el.title
    ? {
        text: el.title,
        left: 'center',
        top: 6,
        textStyle: { fontSize: 15, fontWeight: 600, color: '#0f172a' },
      }
    : undefined

  if (el.chart === 'pie') {
    const series = el.series[0]
    if (!series) return { color: palette, title, series: [] }
    return {
      color: palette,
      title,
      series: [
        {
          type: 'pie',
          radius: '64%',
          center: ['50%', el.title ? '58%' : '52%'],
          data: el.categories.map((name, i) => ({ name, value: series.values[i] })),
          label: { color: '#334155', fontSize: 13 },
          labelLine: { lineStyle: { color: '#94a3b8' } },
        },
      ],
    }
  }

  const legendTop = el.title ? 34 : 6
  return {
    color: palette,
    title,
    grid: { left: 12, right: 20, top: el.title ? 66 : 34, bottom: 8, containLabel: true },
    legend:
      el.series.length > 1
        ? {
            top: legendTop,
            left: 'center',
            itemWidth: 14,
            itemHeight: 9,
            textStyle: { color: '#334155', fontSize: 12 },
          }
        : undefined,
    xAxis: {
      type: 'category',
      data: el.categories,
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#cbd5e1' } },
      axisLabel: { color: '#64748b', fontSize: 12 },
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: '#e2e8f0' } },
      axisLabel: { color: '#64748b', fontSize: 12 },
    },
    series: el.series.map((s) =>
      el.chart === 'line'
        ? { name: s.name, type: 'line', data: s.values, symbolSize: 6 }
        : {
            name: s.name,
            type: 'bar',
            data: s.values,
            barMaxWidth: 36,
            itemStyle: { borderRadius: [3, 3, 0, 0] },
          },
    ),
  }
}

/** chart 距画布底边的最小边距（画布坐标系 px） */
const CHART_BOTTOM_MARGIN = 24

/** 计算图表显示尺寸（画布坐标系）：等比缩放到「不超 el.width、不超画布底边」。 */
function chartSize(el: SlideChartElement, c: SlideContent): { w: number; h: number } {
  const availW = Math.max(el.width, 0)
  const availH = c.height - el.y - CHART_BOTTOM_MARGIN
  if (availW <= 0 || availH <= 0) return { w: availW, h: Math.min(el.height, Math.max(availH, 0)) }
  const s = Math.min(availW / el.width, availH / el.height)
  return { w: el.width * s, h: el.height * s }
}

function chartBox(el: SlideChartElement) {
  const c = content.value
  const size = c ? chartSize(el, c) : { w: el.width, h: el.height }
  return {
    left: px(el.x),
    top: px(el.y),
    width: px(size.w),
    height: px(size.h),
  }
}

/** 内层图表按画布原始尺寸渲染，再整体缩放到目标尺寸（SVG 内容清晰缩放）。
 * 缩放系数必须同时包含画布 fit 缩放（box 宽 = size.w × scale）与超出底边的裁剪比。 */
function chartInner(el: SlideChartElement) {
  const c = content.value
  const size = c ? chartSize(el, c) : { w: el.width, h: el.height }
  const s = el.width > 0 ? (size.w * scale.value) / el.width : 1
  return {
    width: `${el.width}px`,
    height: `${el.height}px`,
    transform: `scale(${s})`,
    transformOrigin: '0 0',
  }
}

// ---- functionPlot 函数图像渲染（动态按需导入 function-plot，SVG 渲染，实例缓存） ----

interface PlotEntry {
  key: string
  chart: Chart | null
}

type FunctionPlotFn = (options: FunctionPlotOptions) => Chart

/** 动态加载 function-plot（CJS 产物）。Vite/esbuild interop 下 default 可能指向
 * module.exports 本体或其 .default 导出，逐层解出真正可调用的绘图函数。 */
async function loadFunctionPlot(): Promise<FunctionPlotFn> {
  const mod = (await import('function-plot')) as { default?: unknown; [key: string]: unknown }
  let fn = (mod?.default ?? mod) as unknown
  if (typeof fn !== 'function') {
    fn = (fn as { default?: unknown } | undefined)?.default
  }
  if (typeof fn !== 'function') {
    throw new Error(
      `cannot resolve functionPlot callable (exports: ${Object.keys(mod ?? {}).join(',')})`,
    )
  }
  return fn as FunctionPlotFn
}

let functionPlotLoader: ReturnType<typeof loadFunctionPlot> | null = null

function loadFunctionPlotOnce() {
  functionPlotLoader ??= loadFunctionPlot()
  return functionPlotLoader
}

/** 元素 id → 绘图实例（元素移除/数据变化时清空 reparent 容器） */
const plotEntries = new Map<string, PlotEntry>()
const plotRefs = new Map<string, HTMLElement>()

function setPlotRef(id: string, node: unknown) {
  if (node instanceof HTMLElement) plotRefs.set(id, node)
  else plotRefs.delete(id)
}

async function renderFunctionPlot(el: SlideFunctionPlotElement, accent: string) {
  const key = `${el.id}@${JSON.stringify({ ...el, id: '', x: 0, y: 0 })}`
  const entry = plotEntries.get(el.id)
  if (entry && entry.key === key) return
  plotEntries.set(el.id, { key, chart: null })
  try {
    await nextTick()
    const dom = plotRefs.get(el.id)
    if (!dom) return
    dom.innerHTML = ''
    // 由封闭的声明字段确定性构建绘图配置（模型只给表达式与窗口），
    // 禁用缩放/平移：slide 内为静态展示，避免与画布交互冲突。
    const functionPlot = await loadFunctionPlotOnce()
    const palette = chartPalette(accent)
    const options: FunctionPlotOptions = {
      target: dom,
      width: el.width,
      height: el.height,
      title: el.title || undefined,
      disableZoom: true,
      grid: el.grid ?? false,
      xAxis: { domain: clampDomain(el.xRange) },
    }
    if (el.yRange && el.yRange[0] < el.yRange[1]) {
      options.yAxis = { domain: [el.yRange[0], el.yRange[1]] }
    }
    options.data = el.curves.map((c, i): FunctionPlotDatum => ({
      fn: c.expression,
      color: c.color || palette[i % palette.length],
      attr: c.dash ? { 'stroke-dasharray': '6 4' } : {},
    }))
    const chart = functionPlot(options)
    plotEntries.set(el.id, { key, chart })
    await nextTick(measureOverlays)
  } catch (e) {
    // 渲染失败：保持 chart 为 null，元素不显示，不影响其余内容（与 chart/mermaid 一致）。
    // 失败原因必须显形：前序实现静默吞错导致"整块不可见"无法定位，这里必须至少警告。
    console.warn('[functionPlot] 渲染失败', e, el.id)
  }
}

function clampDomain(r: [number, number]): [number, number] {
  if (!r || !Number.isFinite(r[0]) || !Number.isFinite(r[1]) || r[0] >= r[1]) return [-6.5, 6.5]
  return r
}

/** functionPlot 与 chart 同款定位：按画布底边等比缩放 */
const PLOT_BOTTOM_MARGIN = 24

function plotSize(el: SlideFunctionPlotElement, c: SlideContent): { w: number; h: number } {
  const availW = Math.max(el.width, 0)
  const availH = c.height - el.y - PLOT_BOTTOM_MARGIN
  if (availW <= 0 || availH <= 0) return { w: availW, h: Math.min(el.height, Math.max(availH, 0)) }
  const s = Math.min(availW / el.width, availH / el.height)
  return { w: el.width * s, h: el.height * s }
}

function plotBox(el: SlideFunctionPlotElement) {
  const c = content.value
  const size = c ? plotSize(el, c) : { w: el.width, h: el.height }
  return {
    left: px(el.x),
    top: px(el.y),
    width: px(size.w),
    height: px(size.h),
  }
}

/** 内层绘图按画布原始尺寸渲染，再整体缩放到目标尺寸（SVG 内容清晰缩放）。
 * 缩放系数与 chartInner 同源：画布 fit 缩放 × 底边裁剪比。 */
function plotInner(el: SlideFunctionPlotElement) {
  const c = content.value
  const size = c ? plotSize(el, c) : { w: el.width, h: el.height }
  const s = el.width > 0 ? (size.w * scale.value) / el.width : 1
  return {
    width: `${el.width}px`,
    height: `${el.height}px`,
    transform: `scale(${s})`,
    transformOrigin: '0 0',
  }
}

// ---- 步骤动作叠加层（underline / highlight / box / laser，多动作）----
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

/** laser 点直径（画布坐标系 px） */
const LASER_SIZE = 14

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
  if (!canvasEl) return
  // 步骤动作 + 讨论模式叠加动作合并渲染（讨论动作随事件实时出现）
  const actions = [...(step?.actions ?? []), ...discussion.overlayActions]
  if (!actions.length) return
  const base = canvasEl.getBoundingClientRect()
  const list: OverlayRect[] = []
  for (const action of actions) {
    // laser：目标元素中心（或画布坐标）画一个红点，仅当前步骤瞬时显示
    if (action.type === 'laser') {
      const size = LASER_SIZE * scale.value
      if (action.targetElementId) {
        const el = elementRefs.get(action.targetElementId)
        if (el) {
          const r = el.getBoundingClientRect()
          list.push({
            key: `${action.type}-${action.targetElementId}`,
            type: action.type,
            left: r.left - base.left + r.width / 2 - size / 2,
            top: r.top - base.top + r.height / 2 - size / 2,
            width: size,
            height: size,
          })
        }
      } else if (action.x != null && action.y != null) {
        list.push({
          key: `${action.type}-xy`,
          type: action.type,
          left: action.x * scale.value - size / 2,
          top: action.y * scale.value - size / 2,
          width: size,
          height: size,
        })
      }
      continue
    }
    if (action.type === 'draw' || action.type === 'clearBoard') {
      continue
    }
    const el = elementRefs.get(action.targetElementId ?? '')
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

// 解析到 mermaid 元素后触发渲染；immediate 保证从非 slide 环节首次挂载
// （组件带着已有 content 被创建，watch 不会再收到变化）时也能渲染。
watch(
  content,
  (c) => {
    if (!c) return
    for (const el of c.elements) {
      if (isMermaid(el)) void renderMermaid(el)
      if (isChart(el)) void renderChart(el, c.accent)
      if (isFunctionPlot(el)) void renderFunctionPlot(el, c.accent)
    }
  },
  { deep: true, immediate: true },
)

onBeforeUnmount(() => {
  for (const entry of chartEntries.values()) entry.inst?.dispose()
  chartEntries.clear()
  plotEntries.clear()
  plotRefs.clear()
})

// 讨论叠加动作变化后重新测量（jump/高亮动作实时落位）
watch(
  () => discussion.overlayActions.map((a) => `${a.type}:${a.targetElementId ?? ''}`).join('|'),
  async () => {
    await nextTick()
    measureOverlays()
  },
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

          <div
            v-else-if="isChart(el)"
            :ref="(n) => setRef(el.id, n)"
            class="stage-el stage-el--chart"
            :style="chartBox(el)"
          >
            <div :ref="(n) => setChartRef(el.id, n)" class="stage-el--chart-inner" :style="chartInner(el)" />
          </div>

          <div
            v-else-if="isFunctionPlot(el)"
            :ref="(n) => setRef(el.id, n)"
            class="stage-el stage-el--function-plot"
            :style="plotBox(el)"
          >
            <div :ref="(n) => setPlotRef(el.id, n)" class="stage-el--function-plot-inner" :style="plotInner(el)" />
          </div>
        </template>

        <div
          v-for="o in overlays"
          :key="o.key"
          class="stage-overlay"
          :class="{
            [`is-${o.type}`]: true,
            'is-laser': o.type === 'laser',
          }"
          :style="{
            left: `${o.left}px`,
            top: `${o.top}px`,
            width: `${o.width}px`,
            height: `${o.height}px`,
            borderColor: content.accent,
            background:
              o.type === 'laser'
                ? '#ef4444'
                : o.type === 'highlight'
                  ? `${content.accent}33`
                  : o.type === 'underline'
                    ? content.accent
                    : undefined,
          }"
        />

        <!-- 电子白板遮罩层：三视图（slide 隐藏 / overlay 透明 / board 白底），
             实际白板内容为声明式笔画回放；讨论侧板另有独立叠加层（不混回放流） -->
        <whiteboard-layer
          :strokes="store.whiteboardStrokes"
          :view="store.manualView"
          :accent="content.accent"
          :canvas-width="content.width"
          :canvas-height="content.height"
          :scale="scale"
          :overlay-strokes="discussion.overlayStrokes"
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

.stage-el--chart {
  overflow: hidden;
}
.stage-el--chart-inner {
  position: absolute;
  left: 0;
  top: 0;
  overflow: hidden;
}
.stage-el--function-plot {
  overflow: hidden;
}
.stage-el--function-plot-inner {
  position: absolute;
  left: 0;
  top: 0;
  overflow: hidden;
}
.stage-el--function-plot-inner :deep(svg) {
  display: block;
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
.stage-overlay.is-laser {
  border: none;
  border-radius: 50%;
  box-shadow: 0 0 8px 4px rgba(239, 68, 68, 0.45);
  animation: laser-pulse 1.6s ease-in-out infinite alternate;
}

@keyframes laser-pulse {
  from {
    opacity: 0.95;
  }
  to {
    opacity: 0.45;
  }
}
</style>
