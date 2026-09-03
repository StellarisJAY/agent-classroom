<script setup lang="ts">
import { NButton, NIcon, NTooltip } from 'naive-ui'
import { ArrowBackOutline } from '@vicons/ionicons5'
import { useRouter } from 'vue-router'

const router = useRouter()

function goHome() {
  router.push('/')
}
</script>

<template>
  <div class="generate-layout">
    <n-tooltip placement="bottom">
      <template #trigger>
        <n-button
          quaternary
          circle
          aria-label="返回课程库"
          class="generate-layout__back"
          @click="goHome"
        >
          <template #icon>
            <n-icon size="20"><ArrowBackOutline /></n-icon>
          </template>
        </n-button>
      </template>
      返回课程库
    </n-tooltip>

    <main class="generate-layout__body">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.generate-layout {
  position: relative;
  height: 100%;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.generate-layout__back {
  position: fixed;
  top: 12px;
  left: 12px;
  z-index: 10;
}

.generate-layout__body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}
</style>
