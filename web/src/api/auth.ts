import { request } from './http'

/** 后端 UserInfo：{ id, username, email } */
export interface UserInfo {
  id: string
  username: string
  email: string
}

/** 注册请求体 */
export interface RegisterPayload {
  username: string
  email: string
  password: string
}

/** 登录请求体（account 可为用户名或邮箱） */
export interface LoginPayload {
  account: string
  password: string
}

/** 登录响应：access token + 用户信息 */
export interface LoginResult {
  token: string
  user: UserInfo
}

/** 注册新用户，返回用户信息 */
export function register(payload: RegisterPayload): Promise<UserInfo> {
  return request<UserInfo>({ url: '/auth/register', method: 'post', data: payload })
}

/** 登录，返回 token + 用户信息 */
export function login(payload: LoginPayload): Promise<LoginResult> {
  return request<LoginResult>({ url: '/auth/login', method: 'post', data: payload })
}

/** 获取当前登录用户 */
export function fetchMe(): Promise<UserInfo> {
  return request<UserInfo>({ url: '/auth/me', method: 'get' })
}
