<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NIcon, NSpin } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'

import ChatPanel from '@/components/learn/ChatPanel.vue'
import LeaveButton from '@/components/learn/LeaveButton.vue'
import SectionToolbar from '@/components/learn/SectionToolbar.vue'
import StageDemo from '@/components/learn/StageDemo.vue'
import StageQuiz from '@/components/learn/StageQuiz.vue'
import StageSlide from '@/components/learn/StageSlide.vue'
import TeacherBar from '@/components/learn/TeacherBar.vue'
import { useConversationStore } from '@/stores/conversation'
import { useLearnStore } from '@/stores/learn'

const route = useRoute()
const store = useLearnStore()
const chat = useConversationStore()

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
  chat.reset()
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
      <span v-if="store.detail" class="learn-view__progress" :data-p="store.progress">
        {{ progressLabel[store.progress] }}
      </span>
    </header>

    <!-- 主舞台 -->
    <main class="learn-view__stage" aria-live="polite">
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
        <StageSlide v-if="store.isSlide" />
        <StageQuiz v-else-if="store.isQuiz" />
        <StageDemo v-else-if="store.isDemo" />
      </template>
    </main>

    <!-- 环节切换工具栏 -->
    <section-toolbar v-if="store.detail" class="learn-view__toolbar" />

    <!-- 底部老师旁白 -->
    <TeacherBar v-if="store.detail" />

    <!-- 问答抽屉 -->
    <ChatPanel />
  </div>
</template>

<style scoped>
.learn-view {
  height: 100%;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.learn-view__header {
  flex: none;
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
}
.learn-view__progress {
  margin-left: auto;
  font-size: 12px;
  padding: 2px 10px;
  border-radius: 999px;
  color: var(--app-text-2, #64748b);
  border: 1px solid var(--app-divider, #e2e8f0);
}
.learn-view__progress[data-p='in_progress'] {
  color: #0d9488;
  border-color: currentColor;
}

.learn-view__stage {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
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

.learn-view__toolbar {
  flex: none;
  padding: 8px 16px;
}
</style>
