<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { NButton, NDrawer, NDrawerContent, NEmpty, NIcon, NInput, NTooltip } from 'naive-ui'
import { PaperPlaneOutline } from '@vicons/ionicons5'

import type { ChatMessage } from '@/api/learn'
import { MessageRole } from '@/api/learn'
import { useIsMobile } from '@/composables/useBreakpoint'
import { useConversationStore } from '@/stores/conversation'
import { useLearnStore } from '@/stores/learn'

const chat = useConversationStore()
const learn = useLearnStore()
const isMobile = useIsMobile()

const draft = ref('')
const bodyEl = ref<HTMLElement | null>(null)
const composerRef = ref<InstanceType<typeof NInput> | null>(null)

const contextHint = () => {
  const s = learn.currentSection
  return s ? `关于「${s.title}」` : '关于本课程'
}

async function scrollToBottom() {
  await nextTick()
  if (bodyEl.value) bodyEl.value.scrollTop = bodyEl.value.scrollHeight
}

watch(
  () => [chat.messages.length, chat.streaming, chat.streamContent],
  () => scrollToBottom(),
)

watch(
  () => chat.open,
  (open) => {
    if (open) {
      scrollToBottom()
      nextTick(() => composerRef.value?.focus())
    }
  },
)

function isUser(m: ChatMessage) {
  return m.role === MessageRole.User
}

async function send() {
  const text = draft.value.trim()
  if (!text || chat.streaming) return
  draft.value = ''
  await chat.ask({
    content: text,
    section_id: learn.currentSection?.id ?? null,
  })
}
</script>

<template>
  <n-drawer
    :show="chat.open"
    :width="isMobile ? '100%' : 420"
    placement="right"
    :z-index="2000"
    @update:show="(v) => (v ? chat.openPanel() : chat.closePanel())"
  >
    <n-drawer-content closable body-content-style="padding:0;display:flex;flex-direction:column">
      <template #header>
        <div class="chat-panel__head">
          <span class="chat-panel__title">智能老师</span>
          <span class="chat-panel__sub">课程级问答 · {{ contextHint() }}</span>
        </div>
      </template>

      <div ref="bodyEl" class="chat-panel__body">
        <div v-if="!chat.messages.length" class="chat-panel__empty">
          <n-empty description="还没有对话，试着问老师一个问题吧" />
        </div>

        <div v-else class="chat-panel__list">
          <div
            v-for="m in chat.messages"
            :key="m.id"
            class="chat-msg"
            :class="isUser(m) ? 'is-user' : 'is-assistant'"
          >
            <span v-if="!isUser(m)" class="chat-msg__avatar" aria-hidden="true">师</span>
            <div class="chat-msg__bubble">{{ m.content }}</div>
          </div>

          <div v-if="chat.streaming" class="chat-msg is-assistant">
            <span class="chat-msg__avatar" aria-hidden="true">师</span>
            <div class="chat-msg__bubble">
              {{ chat.streamContent }}<span class="chat-msg__caret" />
            </div>
          </div>
        </div>
      </div>

      <div class="chat-panel__composer">
        <n-input
          ref="composerRef"
          v-model:value="draft"
          type="textarea"
          :autosize="{ minRows: 1, maxRows: 4 }"
          placeholder="输入你的问题，Enter 发送，Shift+Enter 换行"
          :disabled="chat.streaming"
          @keydown.enter.exact.prevent="send"
        />
        <n-tooltip placement="top">
          <template #trigger>
            <n-button
              circle
              type="primary"
              :disabled="!draft.trim() || chat.streaming"
              :loading="chat.streaming"
              aria-label="发送"
              @click="send"
            >
              <template #icon>
                <n-icon><PaperPlaneOutline /></n-icon>
              </template>
            </n-button>
          </template>
          发送
        </n-tooltip>
      </div>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.chat-panel__head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.chat-panel__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}
.chat-panel__sub {
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}

.chat-panel__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
.chat-panel__empty {
  padding: 48px 0;
}
.chat-panel__list {
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

.chat-panel__composer {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding: 12px 16px;
  padding-bottom: calc(12px + env(safe-area-inset-bottom, 0px));
  border-top: 1px solid var(--app-divider, #e2e8f0);
}
.chat-panel__composer :deep(.n-input) {
  flex: 1;
}

/* ---------- 移动端：气泡占全宽比例更小屏友好 ---------- */
@media (max-width: 768px) {
  .chat-msg__bubble {
    max-width: 88%;
  }
}
</style>
