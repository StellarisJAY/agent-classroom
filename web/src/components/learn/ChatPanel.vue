<script setup lang="ts">
// ChatPanel：Drawer 壳。提问即按方案进入讨论模式（send 后讨论侧板接管；
// 本面板此后可作为历史回看 + 二次追问入口）。
import { ref, watch } from 'vue'
import { NDrawer, NDrawerContent } from 'naive-ui'

import { useIsMobile } from '@/composables/useBreakpoint'
import { useConversationStore } from '@/stores/conversation'
import { useDiscussionStore } from '@/stores/discussion'
import { useLearnStore } from '@/stores/learn'
import ChatMessages from '@/components/learn/ChatMessages.vue'
import ChatComposer from '@/components/learn/ChatComposer.vue'

const chat = useConversationStore()
const discussion = useDiscussionStore()
const learn = useLearnStore()
const isMobile = useIsMobile()

const draft = ref('')
const composerRef = ref<InstanceType<typeof ChatComposer> | null>(null)

const contextHint = () => {
  const s = learn.currentSection
  return s ? `关于「${s.title}」` : '关于本课程'
}

watch(
  () => chat.open,
  (open) => {
    if (open) requestAnimationFrame(() => composerRef.value?.focus())
  },
)

function send(text: string) {
  discussion.start(text)
  // 进入讨论模式后 Drawer 自动让位给侧板
  if (discussion.active) chat.closePanel()
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

      <ChatMessages
        :messages="chat.messages"
        :stream-text="chat.streamContent"
        :error-text="chat.streamError"
      />

      <ChatComposer v-model="draft" :disabled="chat.streaming" @send="send" />
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
</style>
