import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as authApi from '@/api/auth'
import type { LoginPayload, RegisterPayload, UserInfo } from '@/api/auth'
import { clearToken, getToken, setToken } from '@/api/token'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(getToken())
  const user = ref<UserInfo | null>(null)

  const isLoggedIn = computed(() => Boolean(token.value))

  /** 登录：保存 token 与用户信息 */
  async function login(payload: LoginPayload) {
    const res = await authApi.login(payload)
    token.value = res.token
    user.value = res.user
    setToken(res.token)
    return res.user
  }

  /** 注册：注册成功后回填用户信息（暂不自动登录，如需可改） */
  async function register(payload: RegisterPayload) {
    const info = await authApi.register(payload)
    user.value = info
    return info
  }

  /** 拉取当前用户：适合已有 token 时恢复登录态 */
  async function fetchMe() {
    if (!token.value) return null
    const info = await authApi.fetchMe()
    user.value = info
    return info
  }

  /** 退出登录：清 token 与用户态 */
  function logout() {
    token.value = null
    user.value = null
    clearToken()
  }

  return { token, user, isLoggedIn, login, register, fetchMe, logout }
})
