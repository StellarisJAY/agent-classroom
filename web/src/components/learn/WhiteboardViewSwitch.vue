<script setup lang="ts">
// WhiteboardViewSwitch：幻灯片/白板视图三态切换（仅幻灯片 / 幻灯片+板书 / 纯白板）。
// 复用点：StageToolbar 常驻操作区、LearnView 顶栏（讨论模式时工具栏隐藏，顶栏兜底）。
// 纯 UI 状态切换：只读写 learn store 的 manualView / setSlideView，不进入导航锁定语义。
import { NButton, NIcon, NTooltip } from 'naive-ui'
import { CreateOutline, LayersOutline, ReaderOutline } from '@vicons/ionicons5'

import { useLearnStore } from '@/stores/learn'
import type { SlideView } from '@/api/learn'

const store = useLearnStore()

const VIEW_ICONS: { view: SlideView; icon: typeof ReaderOutline; tip: string }[] = [
  { view: 'slide', icon: ReaderOutline, tip: '仅幻灯片' },
  { view: 'overlay', icon: LayersOutline, tip: '幻灯片 + 板书' },
  { view: 'board', icon: CreateOutline, tip: '纯白板' },
]
</script>

<template>
  <div class="wb-view-switch" role="radiogroup" aria-label="白板视图">
    <n-tooltip v-for="v in VIEW_ICONS" :key="v.view" trigger="hover">
      <template #trigger>
        <n-button
          quaternary
          circle
          size="small"
          role="radio"
          :aria-checked="store.manualView === v.view"
          :aria-label="v.tip"
          :type="store.manualView === v.view ? 'primary' : 'default'"
          @click="store.setSlideView(v.view)"
        >
          <template #icon>
            <n-icon :size="17"><component :is="v.icon" /></n-icon>
          </template>
        </n-button>
      </template>
      {{ v.tip }}
    </n-tooltip>
  </div>
</template>

<style scoped>
.wb-view-switch {
  display: flex;
  align-items: center;
  gap: 2px;
}
</style>
