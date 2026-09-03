import { Code } from './types'

/** 业务层统一错误：携带后端业务错误码与 HTTP 状态码，便于组件区分处理 */
export class ApiError extends Error {
  /** 后端业务错误码，见 types.Code */
  code: number
  /** 触发错误的 HTTP 状态码（网络错误时为 0） */
  httpStatus: number

  constructor(code: number, message: string, httpStatus = 0) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.httpStatus = httpStatus
  }

  /** 是否为未登录/登录过期 */
  isUnauthorized(): boolean {
    return this.code === Code.Unauthorized
  }
}
