<script setup lang="ts">
import { computed } from 'vue'
import { NConfigProvider, NDialogProvider, NMessageProvider } from 'naive-ui'

import { useThemeStore } from '@/stores/theme'
import { overridesFor, themeFor } from '@/theme'

const themeStore = useThemeStore()
const theme = computed(() => themeFor(themeStore.theme))
const themeOverrides = computed(() => overridesFor(themeStore.theme))
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
