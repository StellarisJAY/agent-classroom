import { ref } from 'vue'
import { defineStore } from 'pinia'

import * as courseApi from '@/api/course'
import type { GenerationSection, OutlineSection } from '@/api/course'
import { streamGeneration, streamOutline } from '@/api/sse'

/** 大纲生成阶段。 */
export type OutlinePhase = 'idle' | 'generating' | 'generated'
/** 内容生成阶段。idle：大纲已就绪待确认；generating：后台串行生成中；done：完成；error：失败。 */
export type ContentPhase = 'idle' | 'generating' | 'done' | 'error'

/** 生成流程状态（大纲 + 内容两阶段）。 */
export const useGenerationStore = defineStore('generation', () => {
  // ---- 大纲阶段 ----
  const phase = ref<OutlinePhase>('idle')
  const sections = ref<OutlineSection[]>([])
  const title = ref('')
  const error = ref('')
  const busy = ref(false)

  // ---- 内容阶段 ----
  const contentPhase = ref<ContentPhase>('idle')
  const progress = ref<GenerationSection[]>([])
  const contentError = ref('')

  function reset() {
    phase.value = 'idle'
    sections.value = []
    title.value = ''
    error.value = ''
    busy.value = false
    resetContent()
  }

  function resetContent() {
    contentPhase.value = 'idle'
    progress.value = []
    contentError.value = ''
  }

  // ---- 大纲 ----

  /** 拉取已保存大纲；存在则进入 generated，返回 true。 */
  async function loadOutline(courseId: string): Promise<boolean> {
    try {
      const outline = await courseApi.getOutline(courseId)
      sections.value = outline.sections
      phase.value = 'generated'
      return true
    } catch {
      return false
    }
  }

  /** 触发大纲生成，通过 SSE 逐条追加环节。 */
  async function generate(courseId: string): Promise<void> {
    if (busy.value) return
    busy.value = true
    phase.value = 'generating'
    sections.value = []
    title.value = ''
    error.value = ''

    await streamOutline(courseId, {
      onMeta: (t) => {
        title.value = t
      },
      onSection: (s) => {
        sections.value = [...sections.value, s]
      },
      onDone: () => {
        phase.value = 'generated'
      },
      onError: (msg) => {
        error.value = msg
        phase.value = 'idle'
      },
    })
    busy.value = false
  }

  /** 进入生成视图：优先恢复已生成大纲；否则开始生成。 */
  async function initOrGenerate(courseId: string): Promise<void> {
    const has = await loadOutline(courseId)
    if (!has) {
      await generate(courseId)
    }
  }

  // ---- 内容 ----

  /** 将大纲环节（含调序/删除后的结果）提交后端确认并触发后台生成，随后订阅进度。 */
  async function startContent(courseId: string, finalSections: OutlineSection[]): Promise<void> {
    if (contentPhase.value === 'generating') return
    contentPhase.value = 'generating'
    contentError.value = ''
    progress.value = []
    try {
      await courseApi.confirmOutline(courseId, finalSections)
    } catch (e) {
      contentError.value = e instanceof Error ? e.message : '确认大纲失败'
      contentPhase.value = 'error'
      return
    }
    await subscribeContent(courseId)
  }

  /**
   * 进入页面时恢复内容阶段：若课程已确认（存在环节）则读取快照并订阅进度，返回 true；
   * 否则返回 false（大纲待确认，应展示确认按钮）。
   */
  async function resumeContentIfNeeded(courseId: string): Promise<boolean> {
    let secs: GenerationSection[]
    try {
      secs = await courseApi.getSections(courseId)
    } catch {
      secs = []
    }
    if (!secs.length) {
      resetContent()
      return false
    }
    progress.value = secs
    contentPhase.value = secs.every((s) => s.status === courseApi.SectionStatus.Done)
      ? 'done'
      : 'generating'
    await subscribeContent(courseId)
    return true
  }

  /** 订阅内容生成进度，更新 progress 与 contentPhase；流结束/出错后返回。 */
  async function subscribeContent(courseId: string): Promise<void> {
    await streamGeneration(courseId, {
      onSnapshot: (list) => {
        progress.value = list
      },
      onSection: (s, index) => {
        if (index >= 0 && index < progress.value.length) {
          const next = [...progress.value]
          next[index] = s
          progress.value = next
        } else {
          progress.value = [...progress.value, s]
        }
      },
      onCourse: () => {
        contentPhase.value = 'done'
      },
      onDone: () => {
        contentPhase.value = 'done'
      },
      onError: (msg) => {
        contentError.value = msg
        contentPhase.value = 'error'
      },
    })
  }

  return {
    phase,
    sections,
    title,
    error,
    busy,
    contentPhase,
    progress,
    contentError,
    reset,
    resetContent,
    loadOutline,
    generate,
    initOrGenerate,
    startContent,
    resumeContentIfNeeded,
  }
})
