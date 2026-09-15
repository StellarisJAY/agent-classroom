import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as discussionApi from '@/api/discussion'
import type { DiscussionAction } from '@/api/discussion'
import type { SlideAction, SlideStroke } from '@/api/learn'
import { useLearnStore } from '@/stores/learn'
import { useConversationStore } from '@/stores/conversation'
import { BOARD_W, BOARD_H, renormalizeDrawing, sectionCanvasSize } from '@/stores/learnStrokes'

/**
 * 讨论模式状态机（前端侧）：
 * - 进入：提问触发。暂停自动播放、快照 {sectionIndex, stepIndex}、锁定手动导航。
 * - 讨论中：SSE 事件按序消费（严格队列：动作本地执行完毕后才渲染其后的旁白文本）。
 *   元素强调动作渲染叠加层（不入正回放流），jump_to_section 是唯一导航通道。
 * - 退出（终止）：中止流、清叠加层、恢复快照位置、解锁导航。
 */

/** laser 动作的瞬时展示时长（与 StepSlide 的步骤内 laser 语义一致：指一下即走）。 */
const LASER_LINGER_MS = 1200

export const useDiscussionStore = defineStore('discussion', () => {
  const learn = useLearnStore()
  const chat = useConversationStore()

  /** 讨论模式是否激活（进入后到用户点终止为止）。 */
  const active = ref(false)
  /** 本轮提问是否仍在流式接收（end 事件后为 false，可继续追问）。 */
  const streaming = ref(false)
  /** 正在流式输出的旁白文本（未落历史）。 */
  const text = ref('')
  /** 本轮的动作标签（在消息面板内以系统提示展示，窄屏兜底上下文）。 */
  const actionLog = ref<DiscussionAction[]>([])
  /** 讨论叠加层笔画（语义 clear_board 时整层清空），坐标已归一化 1280×720。 */
  const overlayStrokes = ref<SlideStroke[]>([])
  /** 讨论叠加层元素强调动作（highlight/underline/box/laser，渲染在舞台 actor 区）。 */
  const overlayActions = ref<SlideAction[]>([])

  let courseId = ''
  let snapshot: { index: number; stepIndex: number } | null = null
  let controller: AbortController | null = null
  let ended = false

    /** 讨论是否发生在一个可执行舞台动作的环节上（demo 仅文本 + jump）。 */
  const actionCapable = computed(() => learn.isSlide)
  const lastAction = computed(() => actionLog.value[actionLog.value.length - 1] ?? null)

  // ---- 严格队列：SSE 按序到达，但动作有异步落地（jump 等），
  //      用串行链保证"前一个动作落地完成再消费后续文本"。 ----

  let chain: Promise<void> = Promise.resolve()
  function enqueue(task: () => void | Promise<void>): void {
    chain = chain
      .then(task)
      .catch(() => {
        // 单个动作失败静默降级（旁白照讲），方案 §2 边界 2
      })
  }
  async function drain(): Promise<void> {
    const p = chain
    await p
  }

  function init(id: string) {
    courseId = id
  }

  /** 提问并进入讨论模式。流内完成 agent loop；end 后可继续追问。 */
  function start(question: string): void {
    if (active.value && streaming.value) return // 同会话进行中禁并发提问
    if (!courseId) return

    active.value = true
    ended = false
    streaming.value = true
    text.value = ''
    actionLog.value = []
    snapshot = learn.pausePlayback()
    learn.setSectionLocked(true)

    // 讨论首问即会话首消息：直接进会话历史（乐观插入）
    chat.pushUserMessage(question, learn.currentSection?.id ?? null)

    controller = new AbortController()
    void discussionApi.askQuestion(
      courseId,
      { content: question, section_id: learn.currentSection?.id ?? null },
      {
        onText: (delta) => enqueue(() => { text.value += delta }),
        onAction: (action) => enqueue(() => applyAction(action)),
        onEnd: () => enqueue(() => finalizeRound(EndReason.none)),
        onError: (err) => enqueue(() => finalizeRound(EndReason.error, err)),
      },
      controller.signal,
    )
  }

  /** 结束本轮流式回复：增量文本落地为 assistant 消息；讨论仍保持激活。 */
  function finalizeRound(reason: EndReason, err?: unknown): void {
    if (text.value) {
      chat.appendAssistantMessage(text.value, learn.currentSection?.id ?? null)
    }
    text.value = ''
    streaming.value = false
    if (reason === EndReason.error) {
      chat.setStreamError(err) // 面板展示"讨论流中断"
    }
    ended = true
  }

  /** 终止讨论：中止流 → 清叠加层 → 恢复快照 → 解锁导航 → 回到讲解。 */
  function stop(): void {
    if (!active.value) return
    controller?.abort()
    if (!ended) {
      // 流未自然收尾就把已有增量落地为消息（断线/中止不丢上下文）
      finalizeRound(EndReason.user)
    }
    clearOverlay()
    if (snapshot) learn.restorePlayback(snapshot)
    learn.setSectionLocked(false)
    active.value = false
    snapshot = null
    controller = null
  }

  function clearOverlay() {
    overlayStrokes.value = []
    overlayActions.value = []
  }

  // ---- 动作执行（顺序红线：先落地动作，再放行其后的旁白文本） ----

  async function applyAction(action: DiscussionAction): Promise<void> {
    actionLog.value = [...actionLog.value, action]
    switch (action.type) {
      case 'jump_to_section': {
        const idx = learn.sections.findIndex((s) => s.id === action.section_id)
        if (idx >= 0) {
          learn.jumpToSection(idx)
        }
        break
      }
      case 'draw': {
        // 以当前环节画布尺寸归一化到白板坐标系（讨论通常发生在当前环节内）
        const section = learn.currentSection
        const { w, h } = section ? sectionCanvasSize(section) : { w: BOARD_W, h: BOARD_H }
        overlayStrokes.value = [
          ...overlayStrokes.value,
          {
            order: 0, // 叠加层不参与回放排序，仅正回放流依赖 order
            drawing: renormalizeDrawing(action.drawing, BOARD_W / w, BOARD_H / h),
          },
        ]
        break
      }
      case 'clear_board': {
        overlayStrokes.value = []
        break
      }
      case 'laser': {
        // laser 瞬时：存在 overlayActions 渲染后自动消失
        const e: SlideAction = action.targetElementId
          ? { type: 'laser', targetElementId: action.targetElementId }
          : { type: 'laser' }
        overlayActions.value = [...overlayActions.value, e]
        setTimeout(() => {
          overlayActions.value = overlayActions.value.filter((a) => a !== e)
        }, LASER_LINGER_MS)
        break
      }
      default:
        // highlight / underline / box：目标元素叠加（同元素重复动作去重）
        if (!action.targetElementId) break
        overlayActions.value = [
          ...overlayActions.value.filter(
            (a) => !(a.type === action.type && a.targetElementId === action.targetElementId),
          ),
          { type: action.type, targetElementId: action.targetElementId } as SlideAction,
        ]
        break
    }
  }


  /** 页面级 reset（离开学习页时）：清态并中止，不恢复快照。 */
  function reset() {
    controller?.abort()
    clearOverlay()
    active.value = false
    streaming.value = false
    text.value = ''
    actionLog.value = []
    snapshot = null
    controller = null
    ended = false
    chain = Promise.resolve()
  }

  // 暴露 drain 便于测试与组件在退出前确保队列排空
  return {
    active,
    streaming,
    text,
    actionLog,
    overlayStrokes,
    overlayActions,
    actionCapable,
    lastAction,
    init,
    start,
    stop,
    clearOverlay,
    reset,
    drain,
  }
})

/** 流收尾原因：自然端点 / 用户终止（含断线） / 异常。 */
export const EndReason = { none: 'none', user: 'user', error: 'error' } as const
export type EndReason = (typeof EndReason)[keyof typeof EndReason]
