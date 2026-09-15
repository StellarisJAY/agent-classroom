<script setup lang="ts">
// 纯展示消息列表：Drawer 壳与讨论侧板共用。
// 流式中的回复以独立气泡渲染（外层负责提供增量缓冲文本）。
import { nextTick, ref, watch } from 'vue'
import { NEmpty } from 'naive-ui'

import type { ChatMessage } from '@/api/learn'
import { MessageRole } from '@/api/learn'

const props = defineProps<{
  messages: ChatMessage[]
  /** 正在流式输出的增量文本；空串不渲染气泡 */
  streamText?: string
  /** "讨论流中断"提示；空串不渲染 */
  errorText?: string
}>()

const bodyEl = ref<HTMLElement | null>(null)

watch(
  () => [props.messages.length, props.streamText],
  () => void scrollToBottom(),
)

async function scrollToBottom() {
  await nextTick()
  if (bodyEl.value) bodyEl.value.scrollTop = bodyEl.value.scrollHeight
}

defineExpose({ scrollToBottom })

function isUser(m: ChatMessage) {
  return m.role === MessageRole.User
}
</script>

<template>
  <div ref="bodyEl" class="chat-messages__body">
    <div v-if="!messages.length && !(streamText || errorText)" class="chat-messages__empty">
      <n-empty description="还没有对话，试着问老师一个问题吧" />
    </div>

    <div v-else class="chat-messages__list">
      <div
        v-for="m in messages"
        :key="m.id"
        class="chat-msg"
        :class="isUser(m) ? 'is-user' : 'is-assistant'"
      >
        <span v-if="!isUser(m)" class="chat-msg__avatar" aria-hidden="true">师</span>
        <div class="chat-msg__bubble">{{ m.content }}</div>
      </div>

      <div v-if="errorText" class="chat-msg is-assistant">
        <span class="chat-msg__avatar" aria-hidden="true">师</span>
        <div class="chat-msg__bubble chat-msg__bubble--error">{{ errorText }}</div>
      </div>

      <div v-if="streamText" class="chat-msg is-assistant">
        <span class="chat-msg__avatar" aria-hidden="true">师</span>
        <div class="chat-msg__bubble">
          {{ streamText }}<span class="chat-msg__caret" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-messages__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
.chat-messages__empty {
  padding: 48px 0;
}
.chat-messages__list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px 20px;
}

.chat-msg {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
.chat-msg.is-user {
  justify-content: flex-end;
}
.chat-msg__avatar {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  color: #fff;
  background: linear-gradient(135deg, #14b8a6, #0d9488);
  font-size: 12px;
  font-weight: 600;
}
.chat-msg__bubble {
  max-width: 78%;
  padding: 8px 12px;
  font-size: 14px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  border-radius: 10px;
  color: var(--app-text-1, #0f172a);
  background: var(--app-divider, #e2e8f0);
}
.chat-msg__bubble--error {
  color: #b45309;
  background: rgba(245, 158, 11, 0.12);
}
.chat-msg.is-user .chat-msg__bubble {
  color: #fff;
  background: var(--app-primary, #14b8a6);
  border-bottom-right-radius: 2px;
}
.chat-msg.is-assistant .chat-msg__bubble {
  border-bottom-left-radius: 2px;
}
.chat-msg__caret {
  display: inline-block;
  width: 2px;
  height: 1em;
  vertical-align: text-bottom;
  margin-left: 1px;
  background: var(--app-primary, #14b8a6);
  animation: blink 0.9s steps(1) infinite;
}
@keyframes blink {
  50% {
    opacity: 0;
  }
}

@media (max-width: 768px) {
  .chat-msg__bubble {
    max-width: 88%;
  }
}
</style>
