<script setup lang="ts">
// DiscussionPanel：讨论模式侧板（方案 A：讨论期间固定占布局位，无遮罩浮层）。
// 桌面端参与布局，移动端为底部 sheet（上半舞台动作可见）。
// 消息数据流：chat.messages（历史）+ discussion.text（进行中的旁白），
// 动作以标签形式在消息流中展示上下文（动作本体由舞台/白板叠加层渲染）。
import { computed, ref } from 'vue'
import { NButton, NIcon, NTag } from 'naive-ui'
import { StopCircleOutline } from '@vicons/ionicons5'

import type { DiscussionAction } from '@/api/discussion'
import { useConversationStore } from '@/stores/conversation'
import { useDiscussionStore } from '@/stores/discussion'
import { useLearnStore } from '@/stores/learn'
import ChatMessages from '@/components/learn/ChatMessages.vue'
import ChatComposer from '@/components/learn/ChatComposer.vue'

const chat = useConversationStore()
const discussion = useDiscussionStore()
const learn = useLearnStore()

const draft = ref('')

const streamingText = computed(() => (discussion.streaming ? discussion.text : ''))

const actionTexts: Record<DiscussionAction['type'], string> = {
  jump_to_section: '切换环节',
  underline: '划线强调',
  highlight: '高亮重点',
  box: '框选强调',
  laser: '激光笔示意',
  draw: '在白板标记',
  clear_board: '清空白板',
}

const contextHint = computed(() => {
  const s = learn.currentSection
  return s ? `讨论 · ${s.title}` : '讨论'
})

function send(text: string) {
  chat.clearStreamError()
  discussion.start(text)
}
</script>

<template>
  <div class="discussion-panel" aria-label="讨论模式面板">
    <header class="discussion-panel__head">
      <span class="discussion-panel__status">
        <span class="discussion-panel__pulse" aria-hidden="true" />
        讨论模式
        <em>{{ contextHint }}</em>
      </span>
      <n-button
        quaternary
        type="warning"
        size="small"
        aria-label="终止讨论并返回讲解"
        @click="discussion.stop()"
      >
        <template #icon>
          <n-icon><StopCircleOutline /></n-icon>
        </template>
        终止讨论
      </n-button>
    </header>

    <ChatMessages :messages="chat.messages" :stream-text="streamingText" :error-text="chat.streamError" />

    <ul v-if="discussion.actionLog.length" class="discussion-panel__actions" aria-label="老师刚才的动作">
      <li v-for="(a, i) in discussion.actionLog.slice(-6)" :key="i">
        <n-tag size="tiny" :bordered="false" round>{{ actionTexts[a.type] }}</n-tag>
      </li>
    </ul>

    <ChatComposer v-model="draft" :disabled="discussion.streaming" @send="send" />
  </div>
</template>

<style scoped>
.discussion-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: var(--app-header-bg, #ffffff);
}

.discussion-panel__head {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--app-divider, #e2e8f0);
}
.discussion-panel__status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--app-primary, #14b8a6);
}
.discussion-panel__status em {
  font-style: normal;
  font-weight: 400;
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}
.discussion-panel__pulse {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--app-primary, #14b8a6);
  animation: pulse 1.2s ease-in-out infinite;
}
@keyframes pulse {
  50% {
    opacity: 0.35;
  }
}

.discussion-panel__actions {
  flex: none;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin: 0;
  padding: 6px 16px;
  list-style: none;
  border-top: 1px dashed var(--app-divider, #e2e8f0);
}

/* 移动端：整个面板由 LearnView 布局类转为底部 sheet，这里只保内边距紧凑 */
@media (max-width: 768px) {
  .discussion-panel__head {
    padding: 6px 10px;
  }
  .discussion-panel__status em {
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}
</style>
