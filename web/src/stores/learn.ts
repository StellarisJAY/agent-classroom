import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as learnApi from '@/api/learn'
import * as courseApi from '@/api/course'
import type {
  CourseLearnDetail,
  Question,
  SectionLearn,
  SlideAction,
  SlideContent,
  SlideDrawing,
  SlideStep,
  SlideStroke,
  SlideView,
} from '@/api/learn'

/** 内容生成进度轮询间隔。 */
export const CONTENT_POLL_INTERVAL_MS = 3000
/** 环节全部非 generating 且连续无进展次数达到上限判定中断，自动续跑。 */
export const CONTENT_STALL_LIMIT = 15
/** 自动续跑次数上限，超过后转为手动重试横幅。 */
export const CONTENT_MAX_AUTO_RETRY = 5

/** 学习页状态机：课程详情 + 环节/步骤推进 + quiz 作答 + demo 编辑 + 进度。 */
export const useLearnStore = defineStore('learn', () => {
  const courseId = ref('')
  const detail = ref<CourseLearnDetail | null>(null)
  const loading = ref(false)
  const error = ref('')

  // ---- 生成期轮询 ----

  // 内容生成暂停/自动续跑状态
  const stalled = ref(false)
  const autoRetryCount = ref(0)
  const generatedCount = ref(0)
  let genTimer: ReturnType<typeof setTimeout> | null = null
  let stallCount = 0

  // 导航
  const currentIndex = ref(0)
  const lastVisitedIndex = ref(0)

  // slide
  const stepIndex = ref(0)

  // 自动播放
  const autoPlaying = ref(false)
  const playRate = ref(1)
  let autoTimer: ReturnType<typeof setTimeout> | null = null

  // quiz：questionId -> 已选项下标
  const quizAnswers = ref<Record<string, number[]>>({})
  const quizSubmitted = ref(false)

  // demo
  const demoEditing = ref(false)
  const demoDraft = ref('')

  const sections = computed<SectionLearn[]>(() => detail.value?.sections ?? [])
  const currentSection = computed<SectionLearn | null>(
    () => sections.value[currentIndex.value] ?? null,
  )
  const progress = computed(() => detail.value?.progress ?? 'unstarted')

  const isSlide = computed(() => currentSection.value?.type === 'slide')
  const isQuiz = computed(() => currentSection.value?.type === 'quiz')
  const isDemo = computed(() => learnApi.isDemoType(currentSection.value?.type ?? ''))

  /** 生成期状态：当前环节尚未生成完成（舞台应显示转圈而非内容）。 */
  const pendingSection = computed(
    () => !!currentSection.value && currentSection.value.status !== 'done',
  )
  /** 全部环节已生成完成。 */
  const allGenerated = computed(
    () => sections.value.length > 0 && sections.value.every((s) => s.status === 'done'),
  )

  const slideContent = computed<SlideContent | null>(() =>
    isSlide.value ? (currentSection.value!.content as SlideContent) : null,
  )
  const slideSteps = computed<SlideStep[]>(() => (slideContent.value ? slideStepsOf() : []))
  function slideStepsOf(): SlideStep[] {
    const steps = currentSection.value?.steps
    return Array.isArray(steps) ? steps : []
  }

  const stepCount = computed(() => slideSteps.value.length)
  const currentStep = computed<SlideStep | null>(() => slideSteps.value[stepIndex.value] ?? null)

  // ---- 白板 / 遮罩视图 ----
  // 视图是纯用户选择的会话级 UI 状态（AI 动作只控制笔画，不控制视图），
  // 默认遮罩书写，跨步骤/跨环节保持，直到用户再改或刷新页面。

  /** 默认遮罩书写：AI 开始画画时用户不会因为忘记切换而看不到板书。 */
  const manualView = ref<SlideView>('overlay')

  /** 用户手动切换白板视图。 */
  function setSlideView(view: SlideView) {
    manualView.value = view
  }

  /** 当前步骤内的 laser 动作（瞬时显示，不进入笔画序列）。 */
  const laserAction = computed<SlideAction | null>(() => {
    for (const a of currentStep.value?.actions ?? []) {
      if (a.type === 'laser') return a
    }
    return null
  })

  /** 全局笔画重放：白板笔画跨环节累积（order = sectionIndex*1000 + stepIndex），
   *  行进到某位置时取所有 order ≤ 当前位置的 draw 动作按序重放，遇 clearBoard 清空重算。
   *  坐标按产出环节的画布尺寸归一化到 1280×720，跨环节对齐。 */
  const whiteboardStrokes = computed<SlideStroke[]>(() => {
    const out: SlideStroke[] = []
    const secs = sections.value
    const CANVAS_W = 1280
    const CANVAS_H = 720
    for (let si = 0; si < secs.length; si++) {
      const s = secs[si]
      if (!s) continue
      if (si > currentIndex.value) break
      if (s.type !== 'slide' || s.status !== 'done') continue
      const steps = Array.isArray(s.steps) ? s.steps : []
      const limit = si === currentIndex.value ? stepIndex.value : steps.length - 1
      const c = s.content as SlideContent | null
      const cw = c?.width && c.width > 0 ? c.width : 1280
      const ch = c?.height && c.height > 0 ? c.height : 720
      const sx = CANVAS_W / cw
      const sy = CANVAS_H / ch
      for (let j = 0; j <= limit; j++) {
        for (const a of steps[j]?.actions ?? []) {
          if (a.type === 'clearBoard') {
            out.length = 0
          } else if (a.type === 'draw' && a.drawing) {
            out.push({ order: si * 1000 + j, drawing: renormalizeDrawing(a.drawing, sx, sy) })
          }
        }
      }
    }
    return out
  })

  function renormalizeDrawing(d: SlideDrawing, sx: number, sy: number): SlideDrawing {
    if (sx === 1 && sy === 1) return d
    const scalePoints = (pts?: [number, number][]) =>
      pts?.map(([px, py]) => [px * sx, py * sy] as [number, number])
    return {
      ...d,
      points: scalePoints(d.points),
      x: d.x !== undefined ? d.x * sx : undefined,
      y: d.y !== undefined ? d.y * sy : undefined,
      width: d.width !== undefined ? d.width * sx : undefined,
      height: d.height !== undefined ? d.height * sy : undefined,
    }
  }

  const questionCount = computed(() => currentSection.value?.questions.length ?? 0)
  const demoSectionContent = computed(() =>
    isDemo.value ? (currentSection.value!.content as learnApi.DemoContent) : null,
  )
  const demoType = computed(() =>
    isDemo.value ? (currentSection.value!.type as learnApi.SectionTypeValue) : null,
  )

  const hasPrevSection = computed(() => currentIndex.value > 0)
  const hasNextSection = computed(() => currentIndex.value < sections.value.length - 1)

  async function load(id: string) {
    courseId.value = id
    loading.value = true
    error.value = ''
    stopGenPolling()
    try {
      detail.value = await learnApi.getCourseDetail(id)
      reset()
      if (detail.value.progress === 'unstarted') {
        await markProgress('in_progress')
      }
      if (!allGenerated.value) {
        startGenPolling()
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载课程失败'
    } finally {
      loading.value = false
    }
  }

  function reset() {
    stopAutoPlay()
    currentIndex.value = 0
    lastVisitedIndex.value = 0
    stepIndex.value = 0
    quizAnswers.value = {}
    quizSubmitted.value = false
    demoEditing.value = false
  }

  /** 当前请求过的最大环节索引（用于离开时判定是否学完全程） */
  function reachedLast() {
    return sections.value.length > 0 && lastVisitedIndex.value >= sections.value.length - 1
  }

  async function goTo(index: number) {
    stopAutoPlay()
    const clamped = Math.min(Math.max(index, 0), sections.value.length - 1)
    if (clamped === currentIndex.value) return
    currentIndex.value = clamped
    lastVisitedIndex.value = Math.max(lastVisitedIndex.value, clamped)
    stepIndex.value = 0
    quizAnswers.value = {}
    quizSubmitted.value = false
    demoEditing.value = false
  }

  function nextStep() {
    stopAutoPlay()
    if (stepIndex.value < stepCount.value - 1) stepIndex.value += 1
  }

  function prevStep() {
    stopAutoPlay()
    if (stepIndex.value > 0) stepIndex.value -= 1
  }

  // ---- 自动播放 ----

  /** 根据讲解文本估算单步停留时长（每字约 280ms，限幅 1.6s~12s），并按倍速换算。 */
  function estimateDuration(text: string): number {
    const base = Math.min(12000, Math.max(1600, (text?.length ?? 0) * 280))
    return Math.round(base / playRate.value)
  }

  function clearAutoTimer() {
    if (autoTimer) {
      clearTimeout(autoTimer)
      autoTimer = null
    }
  }

  function scheduleNext() {
    clearAutoTimer()
    if (!autoPlaying.value || !isSlide.value) return
    const step = currentStep.value
    if (!step) return
    autoTimer = setTimeout(() => {
      if (stepIndex.value < stepCount.value - 1) {
        stepIndex.value += 1
        scheduleNext()
      } else {
        stopAutoPlay()
      }
    }, estimateDuration(step.text))
  }

  function startAutoPlay() {
    if (!isSlide.value || autoPlaying.value) return
    autoPlaying.value = true
    scheduleNext()
  }

  function stopAutoPlay() {
    autoPlaying.value = false
    clearAutoTimer()
  }

  function toggleAutoPlay() {
    if (autoPlaying.value) stopAutoPlay()
    else startAutoPlay()
  }

  function setPlayRate(rate: number) {
    if (!(rate > 0)) return
    playRate.value = rate
    if (autoPlaying.value) {
      clearAutoTimer()
      scheduleNext()
    }
  }

  // ---- quiz ----

  function toggleOption(questionId: string, optionIndex: number) {
    const question = currentSection.value?.questions.find((q) => q.id === questionId)
    if (!question) return
    const selected = quizAnswers.value[questionId] ?? []
    let next: number[]
    if (question.type === 'single') {
      next = selected.includes(optionIndex) ? [] : [optionIndex]
    } else {
      next = selected.includes(optionIndex)
        ? selected.filter((i) => i !== optionIndex)
        : [...selected, optionIndex]
    }
    quizAnswers.value = { ...quizAnswers.value, [questionId]: next }
  }

  function isSelected(questionId: string, optionIndex: number): boolean {
    return (quizAnswers.value[questionId] ?? []).includes(optionIndex)
  }

  function submitQuiz() {
    quizSubmitted.value = true
  }

  /** 判定单题对错；返回 correct | partial | wrong（仅多选可能出现 partial） */
  function verdict(question: Question): 'correct' | 'partial' | 'wrong' {
    const selected = (quizAnswers.value[question.id] ?? []).slice().sort()
    const answers = question.answers.slice().sort()
    if (!selected.length) return 'wrong'
    if (question.type === 'single') {
      return selected.length === 1 && selected[0] === answers[0] ? 'correct' : 'wrong'
    }
    if (selected.join(',') === answers.join(',')) return 'correct'
    // 多选：选中的都在正确答案内但未选全 → partial
    const allCorrectPicked = selected.every((i) => answers.includes(i))
    return allCorrectPicked ? 'partial' : 'wrong'
  }

  const correctCount = computed(() => {
    const qs = currentSection.value?.questions ?? []
    if (!qs.length) return 0
    return qs.filter((q) => verdict(q) === 'correct').length
  })

  // ---- demo ----

  function startEditing() {
    if (!demoSectionContent.value) return
    demoDraft.value = demoSectionContent.value.code
    demoEditing.value = true
  }

  function cancelEditing() {
    demoEditing.value = false
  }

  async function saveDemo() {
    const section = currentSection.value
    if (!section || !demoEditing.value) return
    await learnApi.saveDemoCode(section.id, demoDraft.value)
    if (section.content && learnApi.isDemoType(section.type)) {
      ;(section.content as learnApi.DemoContent).code = demoDraft.value
    }
    demoEditing.value = false
  }

  // ---- 生成期轮询 ----

  /** 轮询内容生成进度：只合入各环节 status（不动导航/进度）；有环节新完成时重拉详情刷新产物。
   *  检测中断：无 generating 环节且持续无进展 → 自动续跑；超限转手动横幅。 */
  function startGenPolling() {
    if (genTimer !== null) return
    stalled.value = false
    stallCount = 0

    const tick = async () => {
      genTimer = null
      if (!courseId.value || !detail.value) return
      const cur = detail.value

      let secs: courseApi.GenerationSection[] | null = null
      try {
        secs = await courseApi.getSections(courseId.value)
      } catch {
        // 网络抖动：下一轮再试
      }
      if (detail.value !== cur) return
      if (secs) {
        const fresh = new Map(secs.map((s) => [s.id, s]))
        const prevDone = new Set(cur.sections.filter((s) => s.status === 'done').map((s) => s.id))
        let nowDoneCount = 0
        cur.sections = cur.sections.map((s) => {
          const freshSec = fresh.get(s.id)
          const status = freshSec?.status ?? s.status
          if (status === 'done') nowDoneCount += 1
          if (status !== s.status) return { ...s, status }
          return s
        })
        generatedCount.value = nowDoneCount

        // 有环节新完成 → 重拉详情获取最新产物（保留导航与进度）
        const newDone = cur.sections.some(
          (s) => s.status === 'done' && !prevDone.has(s.id),
        )
        if (newDone) {
          try {
            const fd = await learnApi.getCourseDetail(courseId.value)
            if (detail.value === cur) cur.sections = fd.sections
          } catch {
            // 下轮轮询带回新 status
          }
        }
      }

      // 完成即停
      if (detail.value && detail.value.sections.every((s) => s.status === 'done')) {
        generatedCount.value = detail.value.sections.length
        return
      }

      // 中断检测：无 generating 环节且持续无进展 → 自动续跑
      if (detail.value.sections.some((s) => s.status === 'generating')) {
        stallCount = 0
      } else if (++stallCount >= CONTENT_STALL_LIMIT) {
        if (autoRetryCount.value < CONTENT_MAX_AUTO_RETRY) {
          autoRetryCount.value += 1
          stalled.value = false
          try {
            await courseApi.resumeGeneration(courseId.value)
          } catch {
            // 续跑失败下一轮重新计数
          }
          stallCount = 0
        } else {
          stalled.value = true
          return // 停止自动重试，等待手动
        }
      }
      genTimer = setTimeout(tick, CONTENT_POLL_INTERVAL_MS)
    }
    genTimer = setTimeout(tick, CONTENT_POLL_INTERVAL_MS)
  }

  function stopGenPolling() {
    if (genTimer !== null) {
      clearTimeout(genTimer)
      genTimer = null
    }
  }

  /** 手动重试：清零自动重试计数并继续轮询。 */
  async function retryGeneration() {
    if (!courseId.value) return
    autoRetryCount.value = 0
    stalled.value = false
    stallCount = 0
    try {
      await courseApi.resumeGeneration(courseId.value)
    } catch {
      // 轮询下一轮会继续
    }
    startGenPolling()
  }

  // ---- 进度上报 ----

  async function markProgress(status: CourseLearnDetail['progress']) {
    if (!detail.value) return
    detail.value.progress = status
    await learnApi.updateProgress(courseId.value, status)
  }

  return {
    courseId,
    detail,
    loading,
    error,
    sections,
    currentIndex,
    lastVisitedIndex,
    currentSection,
    progress,
    isSlide,
    isQuiz,
    isDemo,
    // 生成期
    pendingSection,
    allGenerated,
    generatedCount,
    autoRetryCount,
    stalled,
    retryGeneration,
    stopGenPolling,
    slideContent,
    slideSteps,
    stepIndex,
    stepCount,
    currentStep,
    manualView,
    setSlideView,
    laserAction,
    whiteboardStrokes,
    questionCount,
    demoSectionContent,
    demoType,
    demoEditing,
    demoDraft,
    autoPlaying,
    playRate,
    hasPrevSection,
    hasNextSection,
    reachedLast,
    load,
    goTo,
    nextStep,
    prevStep,
    toggleAutoPlay,
    setPlayRate,
    toggleOption,
    isSelected,
    quizAnswers,
    quizSubmitted,
    submitQuiz,
    verdict,
    correctCount,
    startEditing,
    cancelEditing,
    saveDemo,
    markProgress,
  }
})
