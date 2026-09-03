<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { NConfigProvider, NDialogProvider, NMessageProvider } from 'naive-ui'

import { useThemeStore } from '@/stores/theme'
import { overridesFor, themeFor } from '@/theme'
import {
  bodyColorFor,
  cardColorFor,
  dividerColorFor,
  primaryColorFor,
  textColor1For,
  textColor2For,
} from '@/theme/tokens'

const themeStore = useThemeStore()
const theme = computed(() => themeFor(themeStore.resolved))
const themeOverrides = computed(() => overridesFor(themeStore.resolved))

// 全局同步：body 背景、原生配色，及自定义元素可用的 --app-* CSS 变量
watchEffect(() => {
  const isDark = themeStore.isDark
  const name = isDark ? 'dark' : 'light'
  const root = document.documentElement
  document.body.style.backgroundColor = bodyColorFor(name)
  root.style.colorScheme = name
  root.style.setProperty('--app-header-bg', bodyColorFor(name))
  root.style.setProperty('--app-card-bg', cardColorFor(name))
  root.style.setProperty('--app-divider', dividerColorFor(name))
  root.style.setProperty('--app-text-1', textColor1For(name))
  root.style.setProperty('--app-text-2', textColor2For(name))
  root.style.setProperty('--app-primary', primaryColorFor(name))
})
</script>

<template>
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
