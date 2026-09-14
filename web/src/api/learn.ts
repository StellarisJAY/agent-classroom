import * as mock from './learn.mock'
import { request } from './http'
import type { ProgressStatusValue } from './course'

/**
 * 学习页领域类型 + 接口签名。
 *
 * 数据获取策略：
 * - slide / quiz 环节由真实接口 GET /courses/:id/learn 提供
 *   （quiz 题目来自后端 question 表）；
 * - demo 后端尚未生成真实内容（仍占位），前端以 learn.mock.ts 兜底；
 * - 问答会话 / 进度上报仍为 mock（待对应接口就位后替换）。
 * 数据 schema 严格对齐 docs/slide数据结构.md 与 docs/数据库设计.md。
 */

// ---- 环节 ----

export const SectionType = {
  Slide: 'slide',
  Quiz: 'quiz',
  Demo3D: 'demo_3d',
  DemoFunction: 'demo_function',
  DemoBasic: 'demo_basic',
} as const
export type SectionTypeValue = (typeof SectionType)[keyof typeof SectionType]

/** 三种 demo 环节类型（聚合判断用） */
export const DemoSectionTypes = [
  SectionType.Demo3D,
  SectionType.DemoFunction,
  SectionType.DemoBasic,
] as const

export function isDemoType(type: string): boolean {
  return (DemoSectionTypes as readonly string[]).includes(type)
}

export const SectionStatus = {
  Pending: 'pending',
  Generating: 'generating',
  Done: 'done',
} as const
export type SectionStatusValue = (typeof SectionStatus)[keyof typeof SectionStatus]

// ---- Slide 画布与元素（docs/slide数据结构.md）----

export type SlideTextAlign = 'left' | 'center' | 'right'

export interface SlideTextElement {
  id: string
  type: 'text'
  x: number
  y: number
  width: number
  content: string
  style: {
    fontSize?: number
    align?: SlideTextAlign
    bold?: boolean
    color?: string
  }
}

export interface SlideFormulaElement {
  id: string
  type: 'formula'
  x: number
  y: number
  content: string
  fontSize?: number
}

export interface SlideShapeElement {
  id: string
  type: 'shape'
  x: number
  y: number
  width: number
  height: number
  shape: 'rect' | 'circle' | 'line' | 'arrow'
  fill?: string
  stroke?: string
  label?: string
}

export interface SlideListElement {
  id: string
  type: 'list'
  x: number
  y: number
  width: number
  ordered?: boolean
  items: string[]
  fontSize?: number
}

export interface SlideImageElement {
  id: string
  type: 'image'
  x: number
  y: number
  width: number
  height: number
  src: string
  prompt?: string
}

export interface SlideMermaidElement {
  id: string
  type: 'mermaid'
  x: number
  y: number
  /** 流程图显示宽度 px，高度按内容自适应 */
  width: number
  /** mermaid 流程图源码（换行以 \n 转义） */
  content: string
  /** 图内文字字号 px，默认 16 */
  fontSize?: number
}

export type SlideChartKind = 'bar' | 'line' | 'pie'

export interface SlideChartSeries {
  name: string
  values: number[]
}

export interface SlideChartElement {
  id: string
  type: 'chart'
  x: number
  y: number
  width: number
  height: number
  /** bar: 数量对比 / line: 趋势 / pie: 占比（固定单系列） */
  chart: SlideChartKind
  title?: string
  /** bar/line 作横轴类目；pie 作扇区名 */
  categories: string[]
  series: SlideChartSeries[]
}

export type SlideElement =
  | SlideTextElement
  | SlideFormulaElement
  | SlideShapeElement
  | SlideListElement
  | SlideImageElement
  | SlideMermaidElement
  | SlideChartElement

export interface SlideContent {
  width: number
  height: number
  background: string
  accent: string
  elements: SlideElement[]
}

export type SlideActionType = 'underline' | 'highlight' | 'box' | 'laser' | 'draw' | 'clearBoard'

/** 白板/遮罩视图（用户手动切换的 UI 状态）：slide 纯幻灯片 | overlay 透明遮罩 | board 不透明白板 */
export type SlideView = 'slide' | 'overlay' | 'board'

export interface SlideAction {
  type: SlideActionType
  targetElementId?: string
  /** laser：画布坐标（画布系绝对坐标）*/
  x?: number
  y?: number
  /** draw：笔画数据 */
  drawing?: SlideDrawing
}

export type SlideDrawKind = 'pen' | 'line' | 'arrow' | 'rect' | 'circle' | 'text'
export type SlideDrawSize = 'thin' | 'medium' | 'thick'

export interface SlideDrawing {
  kind: SlideDrawKind
  size?: SlideDrawSize
  color?: string
  /** pen / line / arrow：折线点集 */
  points?: [number, number][]
  /** rect / circle 包围盒；text 左上角 */
  x?: number
  y?: number
  width?: number
  height?: number
  /** text 内容与字号 */
  content?: string
  fontSize?: number
}

/** 全局回放后的一条白板笔画（前端展示形态） */
export interface SlideStroke {
  /** 全局时序，用于逐笔动画与排序 */
  order: number
  drawing: SlideDrawing
}

export interface SlideStep {
  text: string
  actions?: SlideAction[]
}

// ---- Demo ----
// demo 已按三种类型（demo_3d / demo_function / demo_basic）拆分，具体类型见 section.type；
// content 仅存最终可运行代码（骨架阶段由后端模板拼接产出）。

export interface DemoContent {
  code: string
}

// ---- Quiz ----

export const QuestionType = {
  Single: 'single',
  Multiple: 'multiple',
} as const
export type QuestionTypeValue = (typeof QuestionType)[keyof typeof QuestionType]

export interface Question {
  id: string
  position: number
  type: QuestionTypeValue
  stem: string
  options: string[]
  answers: number[]
  explanations: string[]
}

// ---- 环节（通用结构，按 type 取用对应字段）----

export interface SectionLearn {
  id: string
  position: number
  type: SectionTypeValue
  title: string
  knowledge_points: string[]
  status: SectionStatusValue
  /** slide：SlideContent；demo：DemoContent；其余为 null */
  content: SlideContent | DemoContent | null
  /** slide 讲解步骤；仅 slide 使用 */
  steps: SlideStep[] | null
  /** quiz 题目列表；仅 quiz 使用 */
  questions: Question[]
}

// ---- 课程学习详情 ----

export interface CourseLearnDetail {
  course: { id: string; title: string }
  /** 当前用户对该课程的学习进度（本期后端固定返回 unstarted） */
  progress: ProgressStatusValue
  /** 有序环节列表（按 position） */
  sections: SectionLearn[]
}

// ---- 问答消息 ----

export const MessageRole = {
  User: 'user',
  Assistant: 'assistant',
} as const
export type MessageRoleValue = (typeof MessageRole)[keyof typeof MessageRole]

export interface ChatMessage {
  id: string
  role: MessageRoleValue
  content: string
  section_id: string | null
  created_at: string
}

// ---- 接口签名（后端就位后替换实现）----

export interface AskQuestionInput {
  content: string
  /** 提问来源环节，可空 */
  section_id?: string | null
}

/** 增量回调，真实实现由 SSE 驱动，mock 用计时器驱动 */
export type StreamCallback = (delta: string) => void

// 说明：以下函数目前委托 learn.mock 提供的状态 + 模拟延时/流式，
// 保持与真实后端一致的异步与增量语义。后端接口就位后，仅需把函数体
// 换成 axios / fetch（SSE 读取），签名与调用方不变。

/** 拉取课程学习详情（课程 + 进度 + 有序环节）。
 *  slide / quiz 取接口真实数据；demo 后端未生成则以 mock 兜底。 */
export async function getCourseDetail(courseId: string): Promise<CourseLearnDetail> {
  const detail = await request<CourseLearnDetail>({
    url: `/courses/${courseId}/learn`,
    method: 'get',
  })
  detail.sections = detail.sections.map((s) => {
    // demo_basic 环节已生成完成时兜底注入可运行示例；生成中环节保持 null 以显示转圈。
    if (
      s.type === SectionType.DemoBasic &&
      s.status === SectionStatus.Done &&
      !isDemoContent(s.content)
    ) {
      return { ...s, content: { ...mock.DEMO_CONTENT } }
    }
    return s
  })
  return detail
}

/** 拉取课程级问答历史 */
export async function listMessages(courseId: string): Promise<ChatMessage[]> {
  await delay(150)
  return mock.listMessages(courseId)
}

/** 提问并流式返回老师回复，逐片回调 onDelta */
export async function askQuestion(
  courseId: string,
  input: AskQuestionInput,
  onDelta: StreamCallback,
): Promise<void> {
  const reply = mock.buildAssistantReply(input.content, input.section_id ?? null)
  for (const chunk of splitChunks(reply, 4)) {
    onDelta(chunk)
    await delay(24)
  }
}

/** 上报学习进度 */
export async function updateProgress(courseId: string, status: ProgressStatusValue): Promise<void> {
  await delay(80)
  mock.setProgress(courseId, status)
}

/** 保存 demo 环节代码（唯一可编辑环节） */
export async function saveDemoCode(sectionId: string, code: string): Promise<void> {
  await delay(80)
  mock.setDemoCode(sectionId, code)
}

function delay(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms))
}

/** 判定某环节 content 是否为有效 demo 内容（含 code）。 */
function isDemoContent(content: SectionLearn['content']): content is DemoContent {
  return !!content && typeof content === 'object' && 'code' in content
}

function splitChunks(text: string, size: number): string[] {
  if (!text) return ['']
  const chunks: string[] = []
  for (let i = 0; i < text.length; i += size) chunks.push(text.slice(i, i + size))
  return chunks
}
