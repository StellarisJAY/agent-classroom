import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { THEME_STORAGE_KEY, type ThemeName } from '@/theme'

/** 跟随系统深浅色 */
function systemTheme(): ThemeName {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** 初始化主题：localStorage 优先，其次系统偏好 */
function initTheme(): ThemeName {
  const saved = localStorage.getItem(THEME_STORAGE_KEY)
  return saved === 'light' || saved === 'dark' ? saved : systemTheme()
}

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<ThemeName>(initTheme())

  const isDark = computed(() => theme.value === 'dark')

  function setTheme(next: ThemeName) {
    theme.value = next
    localStorage.setItem(THEME_STORAGE_KEY, next)
  }

  function toggle() {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  return { theme, isDark, setTheme, toggle }
})
