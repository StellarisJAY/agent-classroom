<script setup lang="ts">
// ChatPanel：Drawer 壳。头部承载会话管理（新对话 + 历史会话下拉切换）；
// 提问即按方案进入讨论模式（send 后讨论侧板接管；本面板此后可作为历史回看 + 二次追问入口）。
import { computed, ref, watch } from 'vue'
import { NButton, NDrawer, NDrawerContent, NDropdown, NIcon } from 'naive-ui'
import { Add, FolderOpenOutline } from '@vicons/ionicons5'

import { useIsMobile } from '@/composables/useBreakpoint'
import { useConversationStore } from '@/stores/conversation'
import { useDiscussionStore } from '@/stores/discussion'
import ChatMessages from '@/components/learn/ChatMessages.vue'
import ChatComposer from '@/components/learn/ChatComposer.vue'

const chat = useConversationStore()
const discussion = useDiscussionStore()
const isMobile = useIsMobile()

const draft = ref('')
const composerRef = ref<InstanceType<typeof ChatComposer> | null>(null)

watch(
  () => chat.open,
  (open) => {
    if (open) requestAnimationFrame(() => composerRef.value?.focus())
  },
)

/** 讨论进行/收尾中不允许打断会话切换（叠加层动作可能仍在前端落地）。 */
const sessionLocked = computed(() => !discussion.sessionSwitchable)

function send(text: string) {
  discussion.start(text)
  // 进入讨论模式后 Drawer 自动让位给侧板
  if (discussion.active) chat.closePanel()
}

/** 历史会话下拉项（新对话在登录后的当前会话上操作，不进下拉）。 */
const historyOptions = computed(() =>
  chat.conversations.map((c) => ({
    key: c.id,
    label: c.title || '新对话',
  })),
)

function onOpenNew() {
  if (!sessionLocked.value) chat.newConversation()
}

function onPickHistory(key: string | number) {
  if (sessionLocked.value) return
  void chat.switchConversation(String(key))
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
          <div class="chat-panel__titles">
            <span class="chat-panel__title">智能老师</span>
            <n-dropdown
              trigger="click"
              :options="historyOptions"
              :disabled="sessionLocked"
              @select="onPickHistory"
            >
              <button
                class="chat-panel__history"
                :disabled="sessionLocked"
                type="button"
                title="切换历史会话"
              >
                <n-icon size="13" :component="FolderOpenOutline" />
                <span class="chat-panel__sub chat-panel__cur">{{ chat.activeTitle }}</span>
              </button>
            </n-dropdown>
          </div>
          <n-button
            size="tiny"
            quaternary
            type="primary"
            class="chat-panel__new"
            :disabled="sessionLocked"
            @click="onOpenNew"
          >
            <template #icon><n-icon :component="Add" /></template>
            新对话
          </n-button>
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
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}
.chat-panel__titles {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.chat-panel__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}
.chat-panel__history {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 220px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
  color: var(--app-text-2, #64748b);
  text-align: left;
}
.chat-panel__history:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}
.chat-panel__cur {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chat-panel__sub {
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}
</style>
