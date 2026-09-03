import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as courseApi from '@/api/course'
import type { CourseListItem, CourseScopeValue, ProgressStatusValue } from '@/api/course'

/** 每页卡片数 */
export const COURSE_PAGE_SIZE = 12

/** 课程列表 + 筛选条件（服务端分页）。 */
export const useCourseStore = defineStore('course', () => {
  const keyword = ref('')
  const scope = ref<CourseScopeValue>('all')
  const progress = ref<ProgressStatusValue | ''>('')

  const items = ref<CourseListItem[]>([])
  const total = ref(0)
  const page = ref(1)
  const loading = ref(false)
  /** 发生错误时的提示；成功加载后清空 */
  const error = ref('')

  const hasItems = computed(() => items.value.length > 0)
  /** 是否还有下一页可加载 */
  const hasMore = computed(() => total.value > 0 && page.value * COURSE_PAGE_SIZE < total.value)

  let debounceTimer: ReturnType<typeof setTimeout> | undefined

  /** 按当前筛选条件拉取指定页；replace=true 覆盖本地（翻页），false 追加（加载更多） */
  async function fetchPage(target: number, replace: boolean) {
    loading.value = true
    error.value = ''
    try {
      const res = await courseApi.listCourses({
        scope: scope.value,
        progress: progress.value || undefined,
        keyword: keyword.value.trim() || undefined,
        page: target,
        page_size: COURSE_PAGE_SIZE,
      })
      items.value = replace ? res.items : [...items.value, ...res.items]
      total.value = res.total
      page.value = res.page
      return res
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载课程失败'
      throw e
    } finally {
      loading.value = false
    }
  }

  /** 初次 / 筛选变更后回到第 1 页重新加载 */
  async function reload() {
    page.value = 1
    await fetchPage(1, true)
  }

  /** 翻到指定页 */
  async function goTo(p: number) {
    if (p < 1 || p === page.value) return
    await fetchPage(p, true)
  }

  /** 加载更多（追加到列表尾部） */
  async function loadMore() {
    if (!hasMore.value || loading.value) return
    await fetchPage(page.value + 1, false)
  }

  function setScope(v: CourseScopeValue) {
    if (v === scope.value) return
    scope.value = v
    reload().catch(() => {})
  }

  function setProgress(v: ProgressStatusValue | '') {
    if (v === progress.value) return
    progress.value = v
    reload().catch(() => {})
  }

  /** 名称搜索：防抖 300ms 后回到第 1 页 */
  function setKeyword(v: string) {
    keyword.value = v
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => {
      reload().catch(() => {})
    }, 300)
  }

  function clearError() {
    error.value = ''
  }

  return {
    keyword,
    scope,
    progress,
    items,
    total,
    page,
    loading,
    error,
    hasItems,
    hasMore,
    reload,
    goTo,
    loadMore,
    setScope,
    setProgress,
    setKeyword,
    clearError,
  }
})
