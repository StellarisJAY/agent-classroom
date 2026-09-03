<script setup lang="ts">
import type { TagProps } from 'naive-ui'
import { NButton, NIcon, NTag } from 'naive-ui'
import { ArrowDownOutline, ArrowUpOutline, TrashOutline } from '@vicons/ionicons5'

import { SectionType, type OutlineSection } from '@/api/course'

const props = defineProps<{
  sections: OutlineSection[]
  /** 只读：隐藏操作按钮（如生成中流式阶段） */
  readonly?: boolean
  /** 列表是否有换序能力（首尾隐藏上/下移） */
  reorderable?: boolean
}>()

const emit = defineEmits<{
  moveUp: [index: number]
  moveDown: [index: number]
  remove: [index: number]
}>()

const sectionTypeMeta: Record<string, { label: string; type: TagProps['type'] }> = {
  [SectionType.Slide]: { label: '讲解', type: 'info' },
  [SectionType.Quiz]: { label: '测试', type: 'warning' },
  [SectionType.Demo]: { label: '演示', type: 'success' },
}

function typeMeta(section: OutlineSection) {
  return sectionTypeMeta[section.type] ?? { label: section.type, type: 'default' }
}
</script>

<template>
  <ol class="outline-list">
    <li
      v-for="(s, idx) in props.sections"
      :key="idx"
      class="outline-list__item"
    >
      <div class="outline-list__body">
        <div class="outline-list__row">
          <span class="outline-list__index">{{ idx + 1 }}</span>
          <span class="outline-list__title">{{ s.title }}</span>
          <n-tag size="small" :bordered="false" :type="typeMeta(s).type">
            {{ typeMeta(s).label }}
          </n-tag>
        </div>
        <p v-if="s.knowledge_points?.length" class="outline-list__points">
          {{ s.knowledge_points.join(' · ') }}
        </p>
      </div>

      <div v-if="!readonly" class="outline-list__actions">
        <n-button
          size="tiny"
          quaternary
          :disabled="!reorderable || idx === 0"
          title="上移"
          @click="emit('moveUp', idx)"
        >
          <template #icon>
            <n-icon><ArrowUpOutline /></n-icon>
          </template>
        </n-button>
        <n-button
          size="tiny"
          quaternary
          :disabled="!reorderable || idx === props.sections.length - 1"
          title="下移"
          @click="emit('moveDown', idx)"
        >
          <template #icon>
            <n-icon><ArrowDownOutline /></n-icon>
          </template>
        </n-button>
        <n-button size="tiny" quaternary type="error" title="删除" @click="emit('remove', idx)">
          <template #icon>
            <n-icon><TrashOutline /></n-icon>
          </template>
        </n-button>
      </div>
    </li>
  </ol>
</template>

<style scoped>
.outline-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.outline-list__item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 8px;
  background: var(--app-card-bg, #ffffff);
  animation: fade-in 0.3s ease;
}

.outline-list__body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.outline-list__row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.outline-list__index {
  flex-shrink: 0;
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 12px;
  font-weight: 600;
  color: #fff;
  background: var(--app-primary, #14b8a6);
}

.outline-list__title {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.outline-list__points {
  margin: 0;
  padding-left: 32px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--app-text-2, #64748b);
  word-break: break-word;
}

.outline-list__actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 2px;
}

@keyframes fade-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
