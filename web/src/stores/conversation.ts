import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as learnApi from '@/api/learn'
import type { ChatMessage } from '@/api/learn'
import { MessageRole } from '@/api/learn'

let seq = 0
function uid(): string {
  return `msg-${Date.now()}-${seq++}`
}

/** 课程级问答会话：消息历史 + SSE 流式接收（当前 mock 计时器驱动）。 */
export const useConversationStore = defineStore('conversation', () => {
  const courseId = ref('')
  const messages = ref<ChatMessage[]>([])
  const initialized = ref(false)
  const loading = ref(false)
  /** 面板展开态 */
  const open = ref(false)
  /** 老师正在流式回复 */
  const streaming = ref(false)
  /** 正在生成的增量内容（未落历史前的临时缓冲区） */
  const streamContent = ref('')

  const lastMessage = computed(() => messages.value[messages.value.length - 1] ?? null)

  /** 讨论模式面板展示的"讨论流中断"提示（讨论 store 写入，面板展示后清除）。 */
  const streamError = ref('')

  async function init(id: string) {
    courseId.value = id
    loading.value = true
    try {
      messages.value = await learnApi.listMessages(id)
      initialized.value = true
    } finally {
      loading.value = false
    }
  }

  // ---- 讨论模式写入通道（discussion store 落库用；提问入口统一在 discussion.start） ----

  function openPanel() {
    open.value = true
  }

  function closePanel() {
    open.value = false
  }

  // ---- 讨论模式写入通道（discussion store 落库用；提问入口统一为 discussion.start） ----

  /** 乐观插入一条用户消息（提问即入会话历史）。 */
  function pushUserMessage(content: string, sectionId: string | null): void {
    messages.value.push({
      id: uid(),
      role: MessageRole.User,
      content,
      section_id: sectionId,
      created_at: new Date().toISOString(),
    })
  }

  /** 把一轮回复的完整文本落地为 assistant 消息。 */
  function appendAssistantMessage(content: string, sectionId: string | null): void {
    messages.value.push({
      id: uid(),
      role: MessageRole.Assistant,
      content,
      section_id: sectionId,
      created_at: new Date().toISOString(),
    })
  }

  function setStreamError(err: unknown): void {
    streamError.value = err instanceof Error ? err.message : '讨论流中断，请重新提问'
  }

  function clearStreamError(): void {
    streamError.value = ''
  }

  function reset() {
    courseId.value = ''
    messages.value = []
    initialized.value = false
    streaming.value = false
    streamContent.value = ''
    open.value = false
  }

  return {
    courseId,
    messages,
    initialized,
    loading,
    open,
    streaming,
    streamContent,
    streamError,
    lastMessage,
    init,
    openPanel,
    closePanel,
    pushUserMessage,
    appendAssistantMessage,
    setStreamError,
    clearStreamError,
    reset,
  }
})
