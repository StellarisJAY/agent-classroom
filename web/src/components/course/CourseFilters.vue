<script setup lang="ts">
import { NInput, NSelect } from 'naive-ui'
import type { SelectOption } from 'naive-ui'

import type { CourseScopeValue, ProgressStatusValue } from '@/api/course'
import { CourseScope, ProgressStatus } from '@/api/course'
import { useCourseStore } from '@/stores/course'

/** NSelect 非多选时的取值类型 */
type SelectValue = string | number | Array<string | number> | null

const store = useCourseStore()

const scopeOptions: SelectOption[] = [
  { label: '全部课程', value: CourseScope.All },
  { label: '我创建的', value: CourseScope.Mine },
  { label: '公共库', value: CourseScope.Public },
]

const progressOptions: SelectOption[] = [
  { label: '全部学习状态', value: '' },
  { label: '未学习', value: ProgressStatus.Unstarted },
  { label: '正在学习', value: ProgressStatus.InProgress },
  { label: '已完成', value: ProgressStatus.Completed },
]

function onScopeChange(value: SelectValue) {
  if (typeof value === 'string') store.setScope(value as CourseScopeValue)
}

function onProgressChange(value: SelectValue) {
  if (typeof value === 'string') store.setProgress(value as ProgressStatusValue | '')
}

function handleSearchClick() {
  store.reload().catch(() => {})
}
</script>

<template>
  <div class="course-filters">
    <n-input
      class="course-filters__search"
      :value="store.keyword"
      placeholder="搜索课程名称"
      clearable
      aria-label="按课程名称搜索"
      @update:value="store.setKeyword"
      @keyup.enter="handleSearchClick"
    />
    <n-select
      class="course-filters__select"
      :value="store.scope"
      :options="scopeOptions"
      aria-label="按归属筛选"
      @update:value="onScopeChange"
    />
    <n-select
      class="course-filters__select"
      :value="store.progress"
      :options="progressOptions"
      aria-label="按学习状态筛选"
      @update:value="onProgressChange"
    />
  </div>
</template>

<style scoped>
.course-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.course-filters__search {
  flex: 1;
  min-width: 200px;
  max-width: 360px;
}

.course-filters__select {
  width: 160px;
}
</style>
