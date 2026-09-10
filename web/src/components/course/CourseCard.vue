<script setup lang="ts">
import { computed } from 'vue'
import { NTag } from 'naive-ui'
import type { TagProps } from 'naive-ui'

import type { CourseListItem } from '@/api/course'
import { CourseStatus, ProgressStatus } from '@/api/course'

const props = defineProps<{ item: CourseListItem }>()

interface TagMeta {
  label: string
  type: TagProps['type']
}

const statusMeta = computed<TagMeta>(() => {
  switch (props.item.status) {
    case CourseStatus.Completed:
      return { label: '已完成', type: 'success' }
    case CourseStatus.Generating:
      return { label: '生成中', type: 'info' }
    case CourseStatus.OutlineConfirmed:
      return { label: '待生成', type: 'warning' }
    default:
      return { label: '草稿', type: 'default' }
  }
})

const progressMeta = computed<TagMeta>(() => {
  switch (props.item.progress) {
    case ProgressStatus.Completed:
      return { label: '已完成', type: 'success' }
    case ProgressStatus.InProgress:
      return { label: '正在学习', type: 'primary' }
    default:
      return { label: '未学习', type: 'default' }
  }
})

const sourceLabel = computed(() =>
  props.item.owned ? '我创建的' : props.item.is_public ? '公共库' : '我创建的',
)

/**
 * 内容生成中/已确认大纲（后台生成中）→ 学习页（大纲抽屉显示生成进度，未生成环节转圈）；
 * 草稿/大纲待确认 → 预览生成；已完成 → 进入学习。
 */
const target = computed(() => {
  const s = props.item.status
  if (s === CourseStatus.Draft) return `/preview/${props.item.id}`
  return `/course/${props.item.id}/learn`
})

function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
</script>

<template>
  <router-link :to="target" class="course-card">
    <div class="course-card__head">
      <h3 class="course-card__title">{{ item.title || '未命名课程' }}</h3>
      <n-tag
        :type="sourceLabel === '公共库' ? 'primary' : 'default'"
        size="small"
        :bordered="false"
      >
        {{ sourceLabel }}
      </n-tag>
    </div>

    <div class="course-card__meta">
      <n-tag :type="statusMeta.type" size="small" :bordered="false">{{ statusMeta.label }}</n-tag>
      <n-tag :type="progressMeta.type" size="small" :bordered="false">{{
        progressMeta.label
      }}</n-tag>
      <span class="course-card__date" v-if="formatDate(item.created_at)"
        >创建于 {{ formatDate(item.created_at) }}</span
      >
    </div>
  </router-link>
</template>

<style scoped>
.course-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 8px;
  background-color: var(--app-card-bg, #ffffff);
  text-decoration: none;
  transition:
    border-color 0.2s,
    box-shadow 0.2s,
    transform 0.2s;
}

.course-card:hover,
.course-card:focus-visible {
  border-color: var(--app-primary, #14b8a6);
  outline: none;
}

.course-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.08);
}

.course-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.course-card__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--app-text-1, #0f172a);
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
}

.course-card__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-top: auto;
}

.course-card__date {
  margin-left: auto;
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}
</style>
