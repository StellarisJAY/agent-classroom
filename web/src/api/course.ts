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

/** 后端 CourseListItem */
export interface CourseListItem {
  id: string
  title: string
  description: string
  status: CourseStatusValue
  is_public: boolean
  /** 是否我创建的（owner） */
  owned: boolean
  /** 当前用户对该课程的学习进度 */
  progress: ProgressStatusValue
  created_at: string
  updated_at: string
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
