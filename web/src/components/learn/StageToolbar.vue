<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NDropdown, NIcon, NTooltip } from 'naive-ui'
import {
  ChevronBack,
  ChevronForwardOutline,
  CreateOutline,
  LayersOutline,
  PauseOutline,
  PlayOutline,
  ReaderOutline,
} from '@vicons/ionicons5'

import SectionDrawer from '@/components/learn/SectionDrawer.vue'
import { useLearnStore } from '@/stores/learn'
import type { SlideView } from '@/api/learn'

const store = useLearnStore()

const allAnswered = computed(() => {
  const qs = store.currentSection?.questions ?? []
  if (!qs.length) return false
  return qs.every((q) => (store.quizAnswers[q.id] ?? []).length > 0)
})

const isQuiz = computed(() => store.isQuiz && (store.currentSection?.questions.length ?? 0) > 0)

const rateOptions = [
  { label: '0.5x', key: '0.5' },
  { label: '1x', key: '1' },
  { label: '1.5x', key: '1.5' },
  { label: '2x', key: '2' },
]

const playRateLabel = computed(() => `${store.playRate}x`)

// 白板视图三态图标切换（radio 语义：当前视图按钮高亮）
const VIEW_ICONS: { view: SlideView; icon: typeof ReaderOutline; tip: string }[] = [
  { view: 'slide', icon: ReaderOutline, tip: '仅幻灯片' },
  { view: 'overlay', icon: LayersOutline, tip: '幻灯片 + 板书' },
  { view: 'board', icon: CreateOutline, tip: '纯白板' },
]
</script>

<template>
  <div class="stage-toolbar">
    <div class="stage-toolbar__side">
      <n-button
        quaternary
        size="small"
        :disabled="!store.hasPrevSection"
        @click="store.goTo(store.currentIndex - 1)"
      >
        <template #icon>
          <n-icon><ChevronBack /></n-icon>
        </template>
        上一节
      </n-button>
    </div>

    <div class="stage-toolbar__center">
      <!-- slide：步骤切换 + 自动播放 -->
      <template v-if="store.isSlide">
        <n-button
          quaternary
          size="small"
          :disabled="store.stepIndex === 0"
          @click="store.prevStep()"
        >
          <template #icon>
            <n-icon><ChevronBack /></n-icon>
          </template>
          上一步
        </n-button>

        <n-button
          quaternary
          circle
          size="small"
          :disabled="!store.stepCount"
          :aria-label="store.autoPlaying ? '暂停自动播放' : '自动播放'"
          @click="store.toggleAutoPlay()"
        >
          <template #icon>
            <n-icon :size="18">
              <PauseOutline v-if="store.autoPlaying" />
              <PlayOutline v-else />
            </n-icon>
          </template>
        </n-button>

        <span class="stage-toolbar__counter">
          步骤 {{ store.stepCount ? store.stepIndex + 1 : 0 }} / {{ store.stepCount }}
        </span>

        <n-button
          size="small"
          :disabled="store.stepIndex >= store.stepCount - 1"
          @click="store.nextStep()"
        >
          下一步
          <template #icon>
            <n-icon><ChevronForwardOutline /></n-icon>
          </template>
        </n-button>

        <n-dropdown
          trigger="click"
          :options="rateOptions"
          :value="String(store.playRate)"
          @select="(k) => store.setPlayRate(Number(k))"
        >
          <n-button quaternary size="small" :disabled="!store.stepCount">
            {{ playRateLabel }}
          </n-button>
        </n-dropdown>

        <!-- 白板视图三态切换：仅幻灯片 / 幻灯片+板书 / 纯白板 -->
        <div class="stage-toolbar__views" role="radiogroup" aria-label="白板视图">
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

      <!-- quiz：提交答案 -->
      <n-button
        v-else-if="isQuiz"
        type="primary"
        :disabled="!allAnswered || store.quizSubmitted"
        @click="store.submitQuiz()"
      >
        {{ store.quizSubmitted ? '已提交' : '提交答案' }}
      </n-button>

      <!-- demo / 无题目 quiz：占位 -->
      <span v-else class="stage-toolbar__placeholder" aria-hidden="true" />
    </div>

    <div class="stage-toolbar__side stage-toolbar__side--right">
      <SectionDrawer />

      <n-button
        quaternary
        size="small"
        :disabled="!store.hasNextSection"
        @click="store.goTo(store.currentIndex + 1)"
      >
        下一节
        <template #icon>
          <n-icon><ChevronForwardOutline /></n-icon>
        </template>
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.stage-toolbar {
  flex: 0 0 44px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-top: 1px solid var(--app-divider, #e2e8f0);
}

.stage-toolbar__side {
  flex: 1;
  display: flex;
  align-items: center;
}
.stage-toolbar__side--right {
  justify-content: flex-end;
  gap: 4px;
}

.stage-toolbar__center {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
}

.stage-toolbar__counter {
  font-size: 13px;
  color: var(--app-text-2, #64748b);
  white-space: nowrap;
}

.stage-toolbar__views {
  display: flex;
  align-items: center;
  gap: 2px;
}

.stage-toolbar__placeholder {
  display: inline-block;
}
</style>
