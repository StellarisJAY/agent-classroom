import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import {
  listConversation,
  listConversations,
  type ConversationMessage,
  type ConversationSummary,
} from '@/api/discussion'
import type { ChatMessage } from '@/api/learn'
import { MessageRole } from '@/api/learn'

let seq = 0
function uid(): string {
  return `msg-${Date.now()}-${seq++}`
}

/** 结构化消息 → 面板消息：tool 角色丢弃；assistant 仅取纯文本收尾轮。 */
function toChatMessage(m: ConversationMessage): ChatMessage | null {
  if (m.role === 'tool') return null // 后端合成的工具结果，不渲染
  if (m.role === 'user') {
    return {
      id: m.id || uid(),
      role: MessageRole.User,
      content: m.content.text ?? '',
      section_id: m.section_id ?? null,
      created_at: m.created_at,
    }
  }
  // assistant：带 tool_calls 的是动作轮（无旁白，动作已执行）；仅文本轮可选染
  if (!m.content.text) return null
  return {
    id: m.id || uid(),
    role: MessageRole.Assistant,
    content: m.content.text,
    section_id: null,
    created_at: m.created_at,
  }
}

/** 课程下会话列表切换 + 当前会话：消息历史 + SSE 流式接收。 */
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

  /** 会话列表（按最近活跃倒序，后端排序）。 */
  const conversations = ref<ConversationSummary[]>([])
  /** 当前会话 id；null 表示"新对话"尚未落库（首问后由后端隐式创建）。 */
  const activeId = ref<string | null>(null)

  const lastMessage = computed(() => messages.value[messages.value.length - 1] ?? null)
  /** 当前会话标题（未落库的新会话显示占位）。 */
  const activeTitle = computed(
    () => conversations.value.find((c) => c.id === activeId.value)?.title || '新对话',
  )

  /** 讨论模式面板展示的"讨论流中断"提示（讨论 store 写入，面板展示后清除）。 */
  const streamError = ref('')

  function adaptMessages(raw: ConversationMessage[]): ChatMessage[] {
    return raw.map(toChatMessage).filter((m): m is ChatMessage => m !== null)
  }

  async function init(id: string) {
    courseId.value = id
    loading.value = true
    if (!activeId.value) resetConversation()
    try {
      conversations.value = await listConversations(id).catch(() => [])
      // 默认激活最近活跃会话；无会话保持"新对话"。
      activeId.value = conversations.value[0]?.id ?? null
      if (activeId.value) {
        const raw = await listConversation(id, activeId.value)
        messages.value = adaptMessages(raw)
      }
      initialized.value = true
    } catch {
      messages.value = [] // 历史拉取失败不阻塞进入学习页，讨论仍可提问
      initialized.value = true
    } finally {
      loading.value = false
    }
  }

  /** 刷新会话列表（首问后调用，同步新会话的 id 与标题）。 */
  async function refreshConversations(): Promise<void> {
    if (!courseId.value) return
    try {
      conversations.value = await listConversations(courseId.value)
      // 新会话首问落库后关联其 id（前端在提问前不知道会话 id，由列表头部对齐）。
      if (!activeId.value) activeId.value = conversations.value[0]?.id ?? null
    } catch {
      // 列表刷新失败不影响当前会话继续使用
    }
  }

  /** 新对话：清空当前消息并置空 activeId（首问后 refreshConversations 关联）。 */
  function newConversation(): void {
    activeId.value = null
    messages.value = []
    streamContent.value = ''
    streamError.value = ''
  }

  /** 切换历史会话。 */
  async function switchConversation(id: string): Promise<void> {
    if (id === activeId.value) return
    activeId.value = id
    loading.value = true
    try {
      const raw = await listConversation(courseId.value, id)
      messages.value = adaptMessages(raw)
      streamContent.value = ''
      streamError.value = ''
    } catch {
      messages.value = []
    } finally {
      loading.value = false
    }
  }

  function resetConversation(): void {
    conversations.value = []
    activeId.value = null
    messages.value = []
    streamContent.value = ''
    streamError.value = ''
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
    resetConversation()
    initialized.value = false
    streaming.value = false
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
    conversations,
    activeId,
    activeTitle,
    lastMessage,
    init,
    refreshConversations,
    newConversation,
    switchConversation,
    openPanel,
    closePanel,
    pushUserMessage,
    appendAssistantMessage,
    setStreamError,
    clearStreamError,
    reset,
  }
})
