import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { THEME_STORAGE_KEY, type ThemeName } from '@/theme'

/** 用户主题偏好：跟随系统 / 亮色 / 暗色 */
export type ThemeMode = 'system' | 'light' | 'dark'

/** 读取系统当前是否为深色 */
function systemPrefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

/** 初始化偏好：localStorage 优先，兼容旧版仅 light/dark 的值，默认跟随系统 */
function initMode(): ThemeMode {
  const saved = localStorage.getItem(THEME_STORAGE_KEY)
  return saved === 'light' || saved === 'dark' || saved === 'system' ? saved : 'system'
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>(initMode())

  // 「跟随系统」时实时感知 OS 深浅色变化
  const sysDark = ref(systemPrefersDark())
  const mql = window.matchMedia('(prefers-color-scheme: dark)')
  const onSystemChange = (e: MediaQueryListEvent) => {
    sysDark.value = e.matches
  }
  mql.addEventListener('change', onSystemChange)

  /** 当前实际是否为深色（system 模式跟随 OS） */
  const isDark = computed(() =>
    mode.value === 'system' ? sysDark.value : mode.value === 'dark',
  )

  /** 解析后的主题名，供 Naive UI theme / overrides 使用 */
  const resolved = computed<ThemeName>(() => (isDark.value ? 'dark' : 'light'))

  /** 设置主题偏好并持久化 */
  function setMode(next: ThemeMode) {
    mode.value = next
    localStorage.setItem(THEME_STORAGE_KEY, next)
  }

  return { mode, isDark, resolved, setMode }
})
