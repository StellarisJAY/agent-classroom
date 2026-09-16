import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useConversationStore } from '@/stores/conversation'

// 会话列表/切换的接口 mock：listConversations 返回注入的会话清单，
// listConversation 返回对应会话的消息（以第 4 参原样回传校验切换请求）。
const listConversations =
  vi.fn<(cid: string) => Promise<{ id: string; title: string; update_at: string }[]>>()
const listConversation =
  vi.fn<(cid: string, conversationId?: string | null) => Promise<unknown[]>>()

vi.mock('@/api/discussion', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/discussion')>()),
  listConversations: (cid: string) => listConversations(cid),
  listConversation: (cid: string, conversationId?: string | null) =>
    listConversation(cid, conversationId),
}))

describe('conversation store（多会话管理）', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    listConversations.mockResolvedValue([])
    listConversation.mockResolvedValue([])
  })

  it('init：默认激活最近活跃会话并拉取其历史', async () => {
    listConversations.mockResolvedValue([
      { id: 'conv-2', title: '第二会话', update_at: '2026-01-02T00:00:00Z' },
      { id: 'conv-1', title: '首问标题', update_at: '2026-01-01T00:00:00Z' },
    ])
    const store = useConversationStore()
    await store.init('c1')
    expect(store.activeId).toBe('conv-2')
    expect(listConversation).toHaveBeenCalledWith('c1', 'conv-2')
    expect(store.activeTitle).toBe('第二会话')
  })

  it('init：课程尚无会话时进入"新对话"态', async () => {
    const store = useConversationStore()
    await store.init('c1')
    expect(store.activeId).toBeNull()
    expect(store.activeTitle).toBe('新对话')
    expect(store.messages).toHaveLength(0)
  })

  it('newConversation 清空会话与消息；switchConversation 拉取指定会话历史', async () => {
    listConversations.mockResolvedValue([
      { id: 'conv-1', title: '历史一', update_at: '2026-01-02T00:00:00Z' },
    ])
    const store = useConversationStore()
    await store.init('c1')
    expect(store.activeId).toBe('conv-1')

    // 发过消息后开新对话
    store.pushUserMessage('第一问', null)
    store.newConversation()
    expect(store.activeId).toBeNull()
    expect(store.activeTitle).toBe('新对话')
    expect(store.messages).toHaveLength(0)

    // 切回历史会话
    listConversation.mockResolvedValue([
      {
        id: 'm1',
        role: 'user',
        content: { text: '历史一里的问题' },
        section_id: null,
        created_at: '2026-01-01T00:00:00Z',
      },
    ])
    await store.switchConversation('conv-1')
    expect(store.activeId).toBe('conv-1')
    expect(store.messages).toHaveLength(1)
    expect(store.messages[0]?.content).toBe('历史一里的问题')
  })

  it('refreshConversations 为未落库的新会话回填 id（首问后）', async () => {
    const store = useConversationStore()
    await store.init('c1')
    expect(store.activeId).toBeNull()

    // 模拟首问完成后会话落库出现在列表头部
    listConversations.mockResolvedValue([
      { id: 'conv-new', title: '新会话标题', update_at: '2026-01-03T00:00:00Z' },
    ])
    await store.refreshConversations()
    expect(store.activeId).toBe('conv-new')
    expect(store.activeTitle).toBe('新会话标题')
  })
})
