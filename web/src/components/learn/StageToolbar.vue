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
    <!-- 环节内容操作区（随环节类型变化；移动端独占第一行） -->
    <div class="stage-toolbar__controls">
      <!-- slide：步骤切换 + 自动播放 -->
      <template v-if="store.isSlide">
        <n-button
          quaternary
          size="small"
          :disabled="store.stepIndex === 0"
          data-mobile-icon-only
          @click="store.prevStep()"
        >
          <template #icon>
            <n-icon><ChevronBack /></n-icon>
          </template>
          <span class="stage-toolbar__grow-label">上一步</span>
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
          {{ store.stepCount ? `${store.stepIndex + 1}/${store.stepCount}` : '0/0' }}
        </span>

        <n-button
          size="small"
          :disabled="store.stepIndex >= store.stepCount - 1"
          data-mobile-icon-only
          @click="store.nextStep()"
        >
          <span class="stage-toolbar__grow-label">下一步</span>
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
        size="small"
        :disabled="!allAnswered || store.quizSubmitted"
        @click="store.submitQuiz()"
      >
        {{ store.quizSubmitted ? '已提交' : '提交答案' }}
      </n-button>

      <!-- demo / 无题目 quiz：无内容操作 -->
      <span v-else class="stage-toolbar__placeholder" aria-hidden="true" />
    </div>

    <!-- 环节导航（上一节 / 大纲 / 下一节；移动端独占第二行） -->
    <div class="stage-toolbar__nav-prev">
      <n-button
        quaternary
        size="small"
        :disabled="!store.hasPrevSection"
        data-mobile-icon-only
        @click="store.goTo(store.currentIndex - 1)"
      >
        <template #icon>
          <n-icon><ChevronBack /></n-icon>
        </template>
        <span class="stage-toolbar__grow-label">上一节</span>
      </n-button>
    </div>

    <SectionDrawer />

    <div class="stage-toolbar__nav-next">
      <n-button
        quaternary
        size="small"
        :disabled="!store.hasNextSection"
        data-mobile-icon-only
        @click="store.goTo(store.currentIndex + 1)"
      >
        <span class="stage-toolbar__grow-label">下一节</span>
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
  justify-content: space-between;
  gap: 8px;
  padding: 0 16px;
  border-top: 1px solid var(--app-divider, #e2e8f0);
}

.stage-toolbar__controls {
  flex: none;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.stage-toolbar__counter {
  font-size: 13px;
  color: var(--app-text-2, #64748b);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.stage-toolbar__views {
  display: flex;
  align-items: center;
  gap: 2px;
}

.stage-toolbar__placeholder {
  display: inline-block;
}

/* ---------- 移动端（含小屏横屏）竖屏布局：操作行 + 导航行 ---------- */
@media (max-width: 768px) {
  .stage-toolbar {
    flex: 0 0 auto;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0;
    padding: 0 8px;
    background: var(--app-header-bg, #ffffff);
  }

  /* 第一行：环节内容操作 */
  .stage-toolbar__controls {
    order: -1;
    flex-basis: 100%;
    justify-content: center;
    gap: 4px;
    min-height: 40px;
  }

  /* 第二行：上一节 / 大纲 / 下一节 三等分 */
  .stage-toolbar__nav-prev,
  .stage-toolbar__nav-next {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    height: 40px;
  }
  .stage-toolbar__nav-prev :deep(.n-button),
  .stage-toolbar__nav-next :deep(.n-button) {
    flex: 1;
    justify-content: center !important;
  }
  .stage-toolbar__nav-next {
    justify-content: flex-end;
  }

  /* 第一步操作行的上一步/下一步只留图标 */
  .stage-toolbar__controls .stage-toolbar__grow-label {
    display: none;
  }
  .stage-toolbar__controls :deep(.n-button:not(.n-button--circle)),
  .stage-toolbar__nav-prev :deep(.n-button),
  .stage-toolbar__nav-next :deep(.n-button) {
    padding: 0 6px;
  }
  .stage-toolbar__counter {
    font-size: 12px;
    min-width: 40px;
    text-align: center;
  }
}
</style>
