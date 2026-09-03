/** 后端通用响应结构与错误码，与 internal/types/errors.go 保持一致 */

/** 后端统一响应包 { code, message, data } */
export interface ApiResponse<T = unknown> {
  code: number
  message?: string
  data?: T
}

/** 业务错误码常量（对齐后端 errors.go） */
export const Code = {
  OK: 0,
  BadRequest: 40000,
  Unauthorized: 40100,
  Forbidden: 40300,
  NotFound: 40400,
  Conflict: 40900,
  Validation: 42200,
  Internal: 50000,
} as const

export type CodeValue = (typeof Code)[keyof typeof Code]

/** 登录/注册相关（auth 领域） */
export const AuthError = {
  usernameTaken: Code.Conflict,
  emailTaken: Code.Conflict,
  badCredentials: Code.Unauthorized,
} as const
