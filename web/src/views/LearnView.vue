<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NIcon, NSpin } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'

import ChatPanel from '@/components/learn/ChatPanel.vue'
import DiscussionPanel from '@/components/learn/DiscussionPanel.vue'
import LeaveButton from '@/components/learn/LeaveButton.vue'
import StageDemo from '@/components/learn/StageDemo.vue'
import StageQuiz from '@/components/learn/StageQuiz.vue'
import StageSlide from '@/components/learn/StageSlide.vue'
import StageToolbar from '@/components/learn/StageToolbar.vue'
import TeacherBar from '@/components/learn/TeacherBar.vue'
import WhiteboardViewSwitch from '@/components/learn/WhiteboardViewSwitch.vue'
import { useIsMobile } from '@/composables/useBreakpoint'
import { useConversationStore } from '@/stores/conversation'
import { useDiscussionStore } from '@/stores/discussion'
import { useLearnStore } from '@/stores/learn'

const route = useRoute()
const store = useLearnStore()
const chat = useConversationStore()
const discussion = useDiscussionStore()
const isMobile = useIsMobile()

const courseId = computed(() => String(route.params.courseId ?? ''))

const progressLabel: Record<string, string> = {
  unstarted: '未学习',
  in_progress: '正在学习',
  completed: '已完成',
}

onMounted(() => {
  void load()
})

onBeforeUnmount(() => {
  // 已遍历到最后一个环节则标记完成（mock 阶段进度上报）
  if (store.reachedLast()) {
    store.markProgress('completed').catch(() => {})
  }
  discussion.reset()
  chat.reset()
  store.stopGenPolling()
})

watch(courseId, () => {
  void load()
})

async function load() {
  await store.load(courseId.value)
  await chat.init(courseId.value)
}

function retry() {
  void load()
}
</script>

<template>
  <div class="learn-view">
    <!-- 顶部：返回 + 课程名 -->
    <header class="learn-view__header">
      <leave-button />
      <h1 v-if="store.detail" class="learn-view__title">{{ store.detail.course.title }}</h1>
      <!-- 讨论模式时工具栏隐藏：白板视图切换兜底挂顶栏（PC/移动端共用） -->
      <whiteboard-view-switch
        v-if="store.detail && store.isSlide && discussion.active"
        class="learn-view__wb-switch"
      />
      <span v-if="store.detail" class="learn-view__progress" :data-p="store.progress">
        {{ progressLabel[store.progress] }}
      </span>
    </header>

    <!-- 主舞台：桌面端讨论模式时右侧固定 360px 侧板占布局；移动端侧板转底部 sheet -->
    <main
      class="learn-view__stage"
      :class="{ 'is-discussion': discussion.active && !isMobile, 'is-sheet-open': discussion.active && isMobile }"
      aria-live="polite"
    >
      <n-spin v-if="store.loading" class="learn-view__center" />

      <div v-else-if="store.error" class="learn-view__center">
        <p class="learn-view__error">{{ store.error }}</p>
        <n-button size="small" @click="retry">
          <template #icon>
            <n-icon><RefreshOutline /></n-icon>
          </template>
          重试
        </n-button>
      </div>

      <template v-else-if="store.currentSection">
        <div
          class="learn-view__stage-inner"
          :data-type="store.currentSection.type"
        >
          <!-- 自动重试状态提示 -->
          <p v-if="store.autoRetryCount > 0 && !store.stalled" class="learn-view__autoretry">
            内容生成已中断，已自动重试 {{ store.autoRetryCount }} 次…
          </p>

          <!-- 生成中断：自动重试次数耗尽 → 手动重试横幅 -->
          <div v-if="store.stalled" class="learn-view__stalled">
            <p>内容生成已中断</p>
            <n-button size="small" type="primary" @click="store.retryGeneration()">
              重试继续生成
            </n-button>
          </div>

          <!-- 该环节尚未生成完成 → 转圈等待 -->
          <div v-if="store.pendingSection" class="learn-view__center">
            <n-spin size="medium" />
            <span class="learn-view__pending-hint">该环节正在生成中，请稍候…</span>
          </div>

          <template v-else-if="!store.stalled">
            <StageSlide v-if="store.isSlide" />
            <StageQuiz v-else-if="store.isQuiz" />
            <StageDemo v-else-if="store.isDemo" />
          </template>
        </div>
      </template>

      <!-- 讨论侧板：桌面布局占位（is-discussion 收窄舞台宽），移动端转底部 sheet -->
      <aside
        v-if="discussion.active"
        class="learn-view__discussion"
        :class="{ 'is-sheet': isMobile }"
      >
        <DiscussionPanel />
      </aside>
    </main>

    <!-- 统一工具栏：步骤切换/提交(中) + 大纲(右)；讨论激活时隐藏（移动端本就
         被底部 sheet 覆盖，统一行为给 PC 让出更多对话/舞台空间） -->
    <StageToolbar v-if="store.detail && !discussion.active" />

    <!-- 底部老师旁白（环节生成中或讨论激活时隐藏：讨论旁白由 DiscussionPanel 实时渲染，
         终止入口在 DiscussionPanel 头部） -->
    <TeacherBar v-if="store.detail && !store.pendingSection && !discussion.active" />

    <!-- 问答抽屉 -->
    <ChatPanel />
  </div>
</template>

<style scoped>
.learn-view {
  height: 100%;
  display: flex;
  flex-direction: column;
  /* 移动端讨论 sheet 高度，舞台同量预留 padding 以免内容被盖住 */
  --learn-sheet-h: 46vh;
}

.learn-view__header {
  flex: 0 0 44px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--app-divider, #e2e8f0);
}
.learn-view__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}.learn-view__progress {
  margin-left: auto;
  font-size: 12px;
  padding: 2px 10px;
  border-radius: 999px;
  color: var(--app-text-2, #64748b);
  border: 1px solid var(--app-divider, #e2e8f0);
}
.learn-view__header:has(.learn-view__wb-switch) .learn-view__progress {
  margin-left: 0;
}
.learn-view__progress[data-p='in_progress'] {
  color: #0d9488;
  border-color: currentColor;
}

.learn-view__stage {
  flex: 1 1 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 桌面端讨论模式：舞台 + 固定侧板同行布局（侧板参与布局、无遮罩） */
.learn-view__stage.is-discussion {
  flex-direction: row;
}
.learn-view__stage.is-discussion .learn-view__stage-inner {
  width: auto;
  min-width: 0;
}
.learn-view__discussion {
  flex: 0 0 420px;
  min-height: 0;
  border-left: 1px solid var(--app-divider, #e2e8f0);
}

/* 移动端：底部 sheet（占下半，无遮罩，上半舞台动作仍可见） */
.learn-view__discussion.is-sheet {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  flex: none;
  height: var(--learn-sheet-h, 46vh);
  border-left: none;
  border-top: 1px solid var(--app-divider, #e2e8f0);
  box-shadow: 0 -6px 24px rgba(15, 23, 42, 0.12);
  z-index: 900;
  background: var(--app-header-bg, #ffffff);
}

.learn-view__stage-inner {
  flex: 1;
  min-height: 0;
  width: 100%;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
}
.learn-view__stage-inner[data-type='slide'] {
  max-width: 1200px;
}
.learn-view__stage-inner[data-type='quiz'],
.learn-view__stage-inner[data-type^='demo_'] {
  max-width: 880px;
}

.learn-view__center {
  margin: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 24px;
}
.learn-view__error {
  margin: 0;
  color: var(--app-text-2, #64748b);
}

.learn-view__pending-hint {
  font-size: 14px;
  color: var(--app-text-2, #64748b);
}

.learn-view__autoretry {
  margin: 8px 16px 0;
  font-size: 12px;
  color: var(--app-text-3, #94a3b8);
  text-align: center;
}

.learn-view__stalled {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin: 8px 16px;
  padding: 10px 12px;
  border: 1px solid #f59e0b;
  border-radius: 8px;
  background: rgba(245, 158, 11, 0.08);
}

.learn-view__stalled p {
  margin: 0;
  font-size: 14px;
  color: #b45309;
}

/* ---------- 移动端竖屏：压缩头部 + 舞台可滚动 ---------- */
@media (max-width: 768px) {
  .learn-view__header {
    flex: 0 0 40px;
    gap: 8px;
    padding: 4px 8px;
  }
  .learn-view__title {
    font-size: 14px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .learn-view__progress {
    flex: none;
    font-size: 11px;
    padding: 2px 8px;
  }
  .learn-view__stage {
    overflow-y: auto;
    overflow-x: hidden;
  }
  /* 讨论模式：为底部固定 sheet 预留同高空间，slide/quiz/demo 在剩余上半区居中，不再与 sheet 重合 */
  .learn-view__stage.is-sheet-open {
    padding-bottom: var(--learn-sheet-h);
  }
  .learn-view__stage-inner[data-type='slide'],
  .learn-view__stage-inner[data-type='quiz'],
  .learn-view__stage-inner[data-type^='demo_'] {
    max-width: none;
  }
  .learn-view__stalled {
    flex-wrap: wrap;
    margin: 8px 8px 0;
  }
}
</style>
