<script setup lang="ts">
import { NButton, NIcon, NTooltip } from 'naive-ui'
import { ArrowBackOutline } from '@vicons/ionicons5'
import { useRouter } from 'vue-router'

const router = useRouter()

function leave() {
  router.push('/')
}
</script>

<template>
  <n-tooltip placement="bottom">
    <template #trigger>
      <n-button quaternary circle aria-label="返回课程库" class="leave-button" @click="leave">
        <template #icon>
          <n-icon size="20"><ArrowBackOutline /></n-icon>
        </template>
      </n-button>
    </template>
    返回课程库
  </n-tooltip>
</template>
