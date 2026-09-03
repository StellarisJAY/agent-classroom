<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppNavbar from '@/components/app/AppNavbar.vue'
import SettingsModal from '@/components/app/SettingsModal.vue'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const settingsVisible = ref(false)

onMounted(() => {
  if (authStore.token && !authStore.user) {
    authStore.fetchMe().catch(() => {
      // 拉取失败（如 token 失效）由 http 层统一处理跳登录
    })
  }
})

function openSettings() {
  settingsVisible.value = true
}

function handleLogout() {
  authStore.logout()
  router.replace('/login')
}
</script>

<template>
  <div class="main-layout">
    <app-navbar @open-settings="openSettings" @logout="handleLogout" />
    <main class="main-layout__body">
      <router-view />
    </main>
    <settings-modal v-model:show="settingsVisible" />
  </div>
</template>

<style scoped>
.main-layout {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.main-layout__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
</style>
