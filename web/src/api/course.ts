import { request } from './http'

/** 课程状态：draft | outline_confirmed | generating | completed | failed | partial_failed */
export const CourseStatus = {
  Draft: 'draft',
  OutlineConfirmed: 'outline_confirmed',
  Generating: 'generating',
  Completed: 'completed',
  Failed: 'failed',
  PartialFailed: 'partial_failed',
} as const
export type CourseStatusValue = (typeof CourseStatus)[keyof typeof CourseStatus]

/** 学习进度：unstarted | in_progress | completed */
export const ProgressStatus = {
  Unstarted: 'unstarted',
  InProgress: 'in_progress',
  Completed: 'completed',
} as const
export type ProgressStatusValue = (typeof ProgressStatus)[keyof typeof ProgressStatus]

/** 列表归属筛选：all | mine | public */
export const CourseScope = {
  All: 'all',
  Mine: 'mine',
  Public: 'public',
} as const
export type CourseScopeValue = (typeof CourseScope)[keyof typeof CourseScope]

/** 环节类型：slide | quiz | demo_3d | demo_function | demo_basic */
export const SectionType = {
  Slide: 'slide',
  Quiz: 'quiz',
  Demo3D: 'demo_3d',
  DemoFunction: 'demo_function',
  DemoBasic: 'demo_basic',
} as const
export type SectionTypeValue = (typeof SectionType)[keyof typeof SectionType]

/** 三种 demo 环节类型（聚合判断用） */
export const DemoSectionTypes = [SectionType.Demo3D, SectionType.DemoFunction, SectionType.DemoBasic] as const

export function isDemoType(type: string): boolean {
  return (DemoSectionTypes as readonly string[]).includes(type)
}

/** 大纲状态：draft（待确认）| confirmed */
export const OutlineStatus = {
  Draft: 'draft',
  Confirmed: 'confirmed',
} as const

/** 后端 CourseListItem */
export interface CourseListItem {
  id: string
  title: string
  status: CourseStatusValue
  is_public: boolean
  /** 是否我创建的（owner） */
  owned: boolean
  /** 当前用户对该课程的学习进度 */
  progress: ProgressStatusValue
  created_at: string
  updated_at: string
}

/** 后端 CourseCreateResp */
export interface CourseCreateResult {
  id: string
  title: string
  prompt: string
  status: CourseStatusValue
}

/** 单个大纲环节 */
export interface OutlineSection {
  title: string
  type: SectionTypeValue
  knowledge_points: string[]
  /** 大纲阶段确认/编辑的环节内容描述，确认后作为内容生成的固化要求 */
  description: string
}

/** 环节生成状态：pending | generating | done | failed */
export const SectionStatus = {
  Pending: 'pending',
  Generating: 'generating',
  Done: 'done',
  Failed: 'failed',
} as const
export type SectionStatusValue = (typeof SectionStatus)[keyof typeof SectionStatus]

/** 环节生成进度（确认大纲返回 / SSE 进度） */
export interface GenerationSection {
  id: string
  position: number
  type: SectionTypeValue
  title: string
  status: SectionStatusValue
  knowledge_points: string[]
}

/** 后端 OutlineView（GET /courses/:id/outline） */
export interface OutlineView {
  status: string
  version: number
  sections: OutlineSection[]
}

/** 大纲生成任务状态（GET /courses/:id/outline/task） */
export interface OutlineTaskView {
  status: 'done' | 'generating' | 'error' | 'idle'
  message?: string
  outline?: OutlineView
}

/** 大纲历史版本列表项（POST /courses/:id/outline/versions 返回） */
export interface OutlineVersionView {
  version: number
  title: string
  feedback: string
  current: boolean
  created_at: string
}

/** 列表查询参数（GET query） */
export interface CourseListQuery {
  scope?: CourseScopeValue
  keyword?: string
  progress?: ProgressStatusValue | ''
  page?: number
  page_size?: number
}

/** 后端 CourseListResp */
export interface CourseListResult {
  items: CourseListItem[]
  total: number
  page: number
  page_size: number
}

/** 分页拉取当前用户可见课程 */
export function listCourses(query: CourseListQuery): Promise<CourseListResult> {
  return request<CourseListResult>({ url: '/courses', method: 'get', params: query })
}

/** 思考限制档位（reasoning effort）。与后端 model.Thinking 对齐。 */
export const Thinking = {
  Off: 'off',
  Default: 'default',
  Max: 'max',
} as const
export type ThinkingValue = (typeof Thinking)[keyof typeof Thinking]

/** 大纲环节数量档位（与后端 types 对齐） */
export const OutlineCount = {
  Min: 5,
  Default: 5,
  Max: 30,
} as const

/** 创建课程的模型/思考/大纲环节数/配图选项 */
export interface CourseGenOptions {
  modelConfigId?: string
  thinking?: ThinkingValue
  outlineCount?: number
  /** 是否生成幻灯片配图；为 false 时不传 generate_images */
  generateImages?: boolean
  /** 配图模型配置（kind=image）；为空且开启配图时跟随用户默认 image 配置 */
  imageModelConfigId?: string
}

/** 创建草稿课程（multipart：prompt + files[]，参考文档仅 txt/md） */
export function createCourse(
  prompt: string,
  files: File[],
  opts?: CourseGenOptions,
): Promise<CourseCreateResult> {
  const form = new FormData()
  form.append('prompt', prompt)
  files.forEach((f) => form.append('files', f))
  if (opts?.modelConfigId) form.append('model_config_id', opts.modelConfigId)
  if (opts?.thinking) form.append('thinking', opts.thinking)
  if (typeof opts?.outlineCount === 'number') form.append('outline_count', String(opts.outlineCount))
  if (opts?.generateImages) form.append('generate_images', 'true')
  if (opts?.generateImages && opts.imageModelConfigId)
    form.append('image_model_config_id', opts.imageModelConfigId)
  return request<CourseCreateResult>({ url: '/courses', method: 'post', data: form })
}

/** 查询某课程已保存的大纲；未生成时抛出 404 业务错误 */
export function getOutline(courseId: string): Promise<OutlineView> {
  return request<OutlineView>({ url: `/courses/${courseId}/outline`, method: 'get' })
}

/** 触发大纲后台生成任务（异步）：feedback 可为空（等价全新生成） */
export function startOutline(courseId: string, feedback = ''): Promise<Record<string, never>> {
  return request<Record<string, never>>({
    url: `/courses/${courseId}/outline/regenerate`,
    method: 'post',
    data: { feedback },
  })
}

/** 轮询大纲生成任务状态；status=done 时附带已保存大纲 */
export function getOutlineTask(courseId: string): Promise<OutlineTaskView> {
  return request<OutlineTaskView>({ url: `/courses/${courseId}/outline/task`, method: 'get' })
}

/** 确认大纲：提交最终有序环节列表，后端覆盖大纲并物化，随后后台串行生成内容。
 *  返回物化后的环节生成进度。 */
export function confirmOutline(
  courseId: string,
  sections: OutlineSection[],
): Promise<GenerationSection[]> {
  return request<GenerationSection[]>({
    url: `/courses/${courseId}/outline/confirm`,
    method: 'post',
    data: { sections },
  })
}

/** 查询大纲历史版本列表（最新在前） */
export function listOutlineVersions(courseId: string): Promise<OutlineVersionView[]> {
  return request<OutlineVersionView[]>({ url: `/courses/${courseId}/outline/versions`, method: 'get' })
}

/** 回退大纲到指定历史版本，返回回退后的大纲视图 */
export function revertOutline(courseId: string, version: number): Promise<OutlineView> {
  return request<OutlineView>({
    url: `/courses/${courseId}/outline/revert`,
    method: 'post',
    data: { version },
  })
}

/** 查询某课程全部环节当前生成进度（轮询获取内容生成进度）。 */
export function getSections(courseId: string): Promise<GenerationSection[]> {
  return request<GenerationSection[]>({ url: `/courses/${courseId}/sections`, method: 'get' })
}

/** 恢复内容生成：中断/重启后续跑未完成环节（后端串行跳过已 done）。 */
export function resumeGeneration(courseId: string): Promise<Record<string, never>> {
  return request<Record<string, never>>({
    url: `/courses/${courseId}/generate/resume`,
    method: 'post',
  })
}

/** 重试生成单个失败环节（须等课程生成循环结束后调用）。 */
export function retrySection(
  courseId: string,
  sectionId: string,
): Promise<Record<string, never>> {
  return request<Record<string, never>>({
    url: `/courses/${courseId}/sections/${sectionId}/retry`,
    method: 'post',
  })
}

/** 参考文档提取状态：pending 提取中 / success 成功 / failed 失败 */
export interface DocumentStatus {
  id: string
  filename: string
  extractedStatus: 'pending' | 'success' | 'failed'
}

/** 查询某课程参考文档提取状态（轮询获取提取进度）。 */
export function listDocumentStatus(courseId: string): Promise<DocumentStatus[]> {
  return request<DocumentStatus[]>({ url: `/courses/${courseId}/documents`, method: 'get' })
}
