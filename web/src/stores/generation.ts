import { ref } from 'vue'
import { defineStore } from 'pinia'

import * as courseApi from '@/api/course'
import type { GenerationSection, OutlineSection } from '@/api/course'

/** 大纲生成阶段。 */
export type OutlinePhase = 'idle' | 'generating' | 'generated'
/** 内容生成阶段。idle：大纲已就绪待确认；generating：后台串行生成中；done：完成；error：失败/中断。 */
export type ContentPhase = 'idle' | 'generating' | 'done' | 'error'

/** 大纲任务轮询间隔与网络重试上限。 */
export const OUTLINE_POLL_INTERVAL_MS = 2000
export const OUTLINE_POLL_MAX_RETRY = 3
/** 内容进度轮询间隔。 */
export const CONTENT_POLL_INTERVAL_MS = 3000
/** 环节全部 pending 且连续无进展的次数上限，超过判定为生成中断。 */
export const CONTENT_STALL_LIMIT = 15

/** 生成流程状态（大纲 + 内容两阶段）。 */
export const useGenerationStore = defineStore('generation', () => {
  // ---- 大纲阶段 ----
  const phase = ref<OutlinePhase>('idle')
  const sections = ref<OutlineSection[]>([])
  const title = ref('')
  const error = ref('')
  const busy = ref(false)
  /** 当前生效的课程 ID；用于丢弃已切换课程后旧流/旧轮询的回调。 */
  const activeCourseId = ref('')

  // ---- 内容阶段 ----
  const contentPhase = ref<ContentPhase>('idle')
  const progress = ref<GenerationSection[]>([])
  const contentError = ref('')

  // ---- 轮询计时器 ----
  let outlineTimer: ReturnType<typeof setTimeout> | null = null
  let contentTimer: ReturnType<typeof setTimeout> | null = null

  function reset() {
    stopPolling()
    activeCourseId.value = ''
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

  /** 应用任务返回的大纲视图。 */
  function applyOutline(courseId: string, outline: courseApi.OutlineView) {
    if (courseId !== activeCourseId.value) return
    sections.value = outline.sections
    phase.value = 'generated'
  }

  /** 拉取已保存大纲；存在则进入 generated，返回 true。 */
  async function loadOutline(courseId: string): Promise<boolean> {
    try {
      const outline = await courseApi.getOutline(courseId)
      if (courseId !== activeCourseId.value) return false
      applyOutline(courseId, outline)
      return true
    } catch {
      return false
    }
  }

  /** 触发大纲生成任务并开启轮询；feedback 为空等价全新生成。 */
  async function runOutline(courseId: string, feedback = ''): Promise<void> {
    if (activeCourseId.value !== courseId) return
    if (outlineTimer !== null) return
    stopOutlinePolling()
    busy.value = true
    phase.value = 'generating'
    sections.value = []
    title.value = ''
    error.value = ''

    try {
      await courseApi.startOutline(courseId, feedback)
    } catch (e) {
      error.value = e instanceof Error ? e.message : '大纲生成失败'
      phase.value = 'idle'
      busy.value = false
      return
    }
    busy.value = false
    startOutlinePolling(courseId)
  }

  /** 轮询大纲任务状态，直至 done/error 或切换课程。status=idle 视为未开始/已丢失，提示重试。 */
  function startOutlinePolling(courseId: string) {
    if (outlineTimer !== null) return
    let netRetry = 0

    const tick = async () => {
      if (courseId !== activeCourseId.value) {
        stopOutlinePolling()
        return
      }
      let task: courseApi.OutlineTaskView | null = null
      try {
        task = await courseApi.getOutlineTask(courseId)
        netRetry = 0
      } catch {
        netRetry++
        if (netRetry >= OUTLINE_POLL_MAX_RETRY) {
          error.value = '查询大纲生成状态失败，请重试'
          phase.value = 'idle'
          stopOutlinePolling()
          return
        }
      }
      if (courseId !== activeCourseId.value) {
        stopOutlinePolling()
        return
      }

      switch (task?.status) {
        case 'done': {
          if (task.outline) applyOutline(courseId, task.outline)
          else {
            error.value = '大纲生成异常，请重试'
            phase.value = 'idle'
          }
          stopOutlinePolling()
          return
        }
        case 'error': {
          error.value = task.message || '大纲生成失败，请重试'
          phase.value = 'idle'
          stopOutlinePolling()
          return
        }
        case 'generating':
          // 继续等待
          break
        default: {
          // idle：未开始或服务重启丢失
          error.value = '大纲尚未生成或生成任务已中断，请点击生成'
          phase.value = 'idle'
          stopOutlinePolling()
          return
        }
      }
      outlineTimer = setTimeout(tick, OUTLINE_POLL_INTERVAL_MS)
    }
    outlineTimer = setTimeout(tick, OUTLINE_POLL_INTERVAL_MS)
  }

  function stopOutlinePolling() {
    if (outlineTimer !== null) {
      clearTimeout(outlineTimer)
      outlineTimer = null
    }
  }

  /** 回退大纲到指定历史版本，并重载为当前大纲。 */
  async function revert(courseId: string, version: number): Promise<void> {
    const outline = await courseApi.revertOutline(courseId, version)
    if (courseId !== activeCourseId.value) return
    applyOutline(courseId, outline)
    title.value = ''
  }

  /** 拉取大纲历史版本列表。 */
  async function listVersions(courseId: string): Promise<courseApi.OutlineVersionView[]> {
    return courseApi.listOutlineVersions(courseId)
  }

  /** 进入生成视图：切换课程时重置旧状态；优先恢复已生成大纲，否则触发后台生成并轮询。 */
  async function initOrGenerate(courseId: string): Promise<void> {
    if (courseId !== activeCourseId.value) {
      reset()
    }
    activeCourseId.value = courseId
    const has = await loadOutline(courseId)
    if (!has) {
      await runOutline(courseId, '')
    }
  }

  // ---- 内容 ----

  /** 将大纲环节（含调序/删除后的结果）提交后端确认并触发后台生成，随后轮询进度。 */
  async function startContent(courseId: string, finalSections: OutlineSection[]): Promise<void> {
    if (contentPhase.value === 'generating') return
    activeCourseId.value = courseId
    contentPhase.value = 'generating'
    contentError.value = ''
    progress.value = []
    try {
      const progs = await courseApi.confirmOutline(courseId, finalSections)
      if (courseId === activeCourseId.value) progress.value = progs
    } catch (e) {
      contentError.value = e instanceof Error ? e.message : '确认大纲失败'
      contentPhase.value = 'error'
      return
    }
    startContentPolling(courseId)
  }

  /** 恢复内容生成（中断/重启后续跑），随后轮询进度。 */
  async function resumeGeneration(courseId: string): Promise<void> {
    activeCourseId.value = courseId
    contentError.value = ''
    contentPhase.value = 'generating'
    try {
      await courseApi.resumeGeneration(courseId)
    } catch (e) {
      contentError.value = e instanceof Error ? e.message : '恢复生成失败'
      contentPhase.value = 'error'
      return
    }
    startContentPolling(courseId)
  }

  /**
   * 进入页面时恢复内容阶段：若课程已确认（存在环节）则读取快照并进入轮询，返回 true；
   * 否则返回 false（大纲待确认，应展示确认按钮）。
   */
  async function resumeContentIfNeeded(courseId: string): Promise<boolean> {
    let secs: GenerationSection[]
    try {
      secs = await courseApi.getSections(courseId)
    } catch {
      secs = []
    }
    if (courseId !== activeCourseId.value) return false
    activeCourseId.value = courseId
    if (!secs.length) {
      resetContent()
      return false
    }
    activeCourseId.value = courseId
    progress.value = secs
    contentPhase.value = secs.every((s) => s.status === courseApi.SectionStatus.Done)
      ? 'done'
      : 'generating'
    startContentPolling(courseId)
    return true
  }

  /**
   * 轮询内容生成进度：全部 done → done；有环节 generating 或已有部分完成 → 继续；
   * 全部 pending 且持续无进展 → 判定中断，提示重试。
   */
  function startContentPolling(courseId: string) {
    if (contentTimer !== null) return
    let stallCount = 0

    const tick = async () => {
      if (courseId !== activeCourseId.value) {
        stopContentPolling()
        return
      }
      let ok = true
      try {
        const secs = await courseApi.getSections(courseId)
        if (courseId !== activeCourseId.value) {
          stopContentPolling()
          return
        }
        progress.value = secs
        if (secs.length && secs.every((s) => s.status === courseApi.SectionStatus.Done)) {
          contentPhase.value = 'done'
          stopContentPolling()
          return
        }
        if (secs.some((s) => s.status !== courseApi.SectionStatus.Pending)) {
          stallCount = 0
        } else if (++stallCount >= CONTENT_STALL_LIMIT) {
          contentError.value = '内容生成已中断，请点击重试'
          contentPhase.value = 'error'
          stopContentPolling()
          return
        }
      } catch {
        ok = false
      }
      if (!ok) {
        contentError.value = contentError.value || '查询生成进度失败，请重试'
        contentPhase.value = 'error'
        stopContentPolling()
        return
      }
      contentTimer = setTimeout(tick, CONTENT_POLL_INTERVAL_MS)
    }
    contentTimer = setTimeout(tick, CONTENT_POLL_INTERVAL_MS)
  }

  function stopContentPolling() {
    if (contentTimer !== null) {
      clearTimeout(contentTimer)
      contentTimer = null
    }
  }

  /** 停止全部轮询（页面卸载/切课时调用）。 */
  function stopPolling() {
    stopOutlinePolling()
    stopContentPolling()
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
    stopPolling,
    loadOutline,
    generate: runOutline,
    regenerate: runOutline,
    revert,
    listVersions,
    initOrGenerate,
    startContent,
    resumeContentIfNeeded,
    resumeGeneration,
  }
})
