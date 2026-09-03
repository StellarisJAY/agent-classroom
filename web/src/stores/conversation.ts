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

  async function ask(input: { content: string; section_id?: string | null }) {
    if (!courseId.value || streaming.value) return
    if (!initialized.value) await init(courseId.value)

    const user: ChatMessage = {
      id: uid(),
      role: MessageRole.User,
      content: input.content,
      section_id: input.section_id ?? null,
      created_at: new Date().toISOString(),
    }
    messages.value.push(user)

    open.value = true
    streaming.value = true
    streamContent.value = ''
    try {
      await learnApi.askQuestion(courseId.value, input, (delta) => {
        streamContent.value += delta
      })
    } finally {
      if (streamContent.value) {
        messages.value.push({
          id: uid(),
          role: MessageRole.Assistant,
          content: streamContent.value,
          section_id: input.section_id ?? null,
          created_at: new Date().toISOString(),
        })
      }
      streamContent.value = ''
      streaming.value = false
    }
  }

  function openPanel() {
    open.value = true
  }

  function closePanel() {
    open.value = false
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
    lastMessage,
    init,
    ask,
    openPanel,
    closePanel,
    reset,
  }
})
