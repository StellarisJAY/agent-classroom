import axios, { AxiosError, type AxiosRequestConfig } from 'axios'

import { ApiError } from './error'
import { clearToken, getToken } from './token'
import type { ApiResponse } from './types'
import { Code } from './types'

/** 未登录跳转路径（登录页占位路由） */
const LOGIN_PATH = '/login'

/** axios 实例：baseURL 走 /api，由 vite dev 代理转发到后端 */
const http = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

// 请求拦截：附加 Bearer token
http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截：解包统一响应 + 归一化错误。后端业务失败统一 200 + code，因此 401 主路径在成功分支。
http.interceptors.response.use(
  (response) => {
    const body = response.data as ApiResponse
    if (body && typeof body.code === 'number' && body.code !== Code.OK) {
      const apiErr = new ApiError(body.code, body.message ?? '请求失败', response.status)
      if (apiErr.isUnauthorized()) handleUnauthorized()
      return Promise.reject(apiErr)
    }
    return response
  },
  (error: AxiosError<ApiResponse>) => {
    let apiErr: ApiError
    if (error.response) {
      // 兜底：网关/代理等返回非 2xx 且带响应体的情况
      const body = error.response.data
      const code = body?.code ?? httpStatusToCode(error.response.status)
      const message = body?.message ?? fallbackMessage(error.response.status)
      apiErr = new ApiError(code, message, error.response.status)
    } else if (error.code === 'ECONNABORTED') {
      apiErr = new ApiError(Code.Internal, '请求超时，请稍后重试')
    } else {
      apiErr = new ApiError(Code.Internal, '网络异常，请检查网络连接')
    }

    // 全局 401：清除登录态并跳登录页
    if (apiErr.isUnauthorized()) handleUnauthorized()
    return Promise.reject(apiErr)
  },
)

/** 泛型请求：直接返回业务数据 data */
export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const response = await http.request<ApiResponse<T>>(config)
  return response.data.data as T
}

/** 无返回数据的请求，仍校验 code */
export async function requestVoid(config: AxiosRequestConfig): Promise<void> {
  await request<undefined>(config)
}

function handleUnauthorized(): void {
  clearToken()
  redirectToLogin()
}

function redirectToLogin(): void {
  if (window.location.pathname !== LOGIN_PATH) {
    // 整页跳转以重置内存中的用户态，避免 http→store 循环依赖
    window.location.href = LOGIN_PATH
  }
}

function httpStatusToCode(status: number): number {
  const map: Record<number, number> = {
    400: Code.BadRequest,
    401: Code.Unauthorized,
    403: Code.Forbidden,
    404: Code.NotFound,
    409: Code.Conflict,
    422: Code.Validation,
    500: Code.Internal,
  }
  return map[status] ?? Code.Internal
}

function fallbackMessage(status: number): string {
  const map: Record<number, string> = {
    400: '请求参数不合法',
    401: '未登录或登录已过期',
    403: '无权访问',
    404: '资源不存在',
    409: '数据冲突',
    422: '参数校验失败',
    500: '服务器内部错误',
  }
  return map[status] ?? '请求失败'
}

export default http
