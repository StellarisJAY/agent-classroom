import { ref } from 'vue'
import { defineStore } from 'pinia'

import * as courseApi from '@/api/course'
import type { OutlineSection } from '@/api/course'
import { streamOutline } from '@/api/sse'

/** 生成流程状态。 */
export const useGenerationStore = defineStore('generation', () => {
  /** idle: 尚未开始；generating: 生成中；generated: 大纲已就绪 */
  const phase = ref<'idle' | 'generating' | 'generated'>('idle')
  const sections = ref<OutlineSection[]>([])
  const title = ref('')
  const error = ref('')
  const busy = ref(false)

  function reset() {
    phase.value = 'idle'
    sections.value = []
    title.value = ''
    error.value = ''
    busy.value = false
  }

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

  return {
    phase,
    sections,
    title,
    error,
    busy,
    reset,
    loadOutline,
    generate,
    initOrGenerate,
  }
})
