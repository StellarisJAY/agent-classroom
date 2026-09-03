import { request } from './http'

/** 课程状态：draft | outline_confirmed | generating | completed */
export const CourseStatus = {
  Draft: 'draft',
  OutlineConfirmed: 'outline_confirmed',
  Generating: 'generating',
  Completed: 'completed',
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

/** 环节类型：slide | quiz | demo */
export const SectionType = {
  Slide: 'slide',
  Quiz: 'quiz',
  Demo: 'demo',
} as const
export type SectionTypeValue = (typeof SectionType)[keyof typeof SectionType]

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
}

/** 环节生成状态：pending | generating | done */
export const SectionStatus = {
  Pending: 'pending',
  Generating: 'generating',
  Done: 'done',
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
  sections: OutlineSection[]
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

/** 创建课程的模型/思考选项 */
export interface CourseGenOptions {
  modelConfigId?: string
  thinking?: ThinkingValue
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
  return request<CourseCreateResult>({ url: '/courses', method: 'post', data: form })
}

/** 查询某课程已保存的大纲；未生成时抛出 404 业务错误 */
export function getOutline(courseId: string): Promise<OutlineView> {
  return request<OutlineView>({ url: `/courses/${courseId}/outline`, method: 'get' })
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

/** 查询某课程全部环节当前生成进度（用于进入页面时恢复/推导步骤）。 */
export function getSections(courseId: string): Promise<GenerationSection[]> {
  return request<GenerationSection[]>({ url: `/courses/${courseId}/sections`, method: 'get' })
}
