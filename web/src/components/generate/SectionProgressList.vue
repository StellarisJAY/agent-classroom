<script setup lang="ts">
import type { TagProps } from 'naive-ui'
import { NButton, NIcon, NSpin, NTag } from 'naive-ui'
import { CheckmarkCircleOutline, CloseCircleOutline, SyncOutline } from '@vicons/ionicons5'

import { SectionStatus, SectionType, type GenerationSection } from '@/api/course'

defineProps<{ sections: GenerationSection[] }>()

const emit = defineEmits<{ retry: [sectionId: string] }>()

const sectionTypeMeta: Record<string, { label: string; type: TagProps['type'] }> = {
  [SectionType.Slide]: { label: '讲解', type: 'info' },
  [SectionType.Quiz]: { label: '测试', type: 'warning' },
  [SectionType.Demo3D]: { label: '3D 演示', type: 'success' },
  [SectionType.DemoFunction]: { label: '函数演示', type: 'success' },
  [SectionType.DemoBasic]: { label: '基础演示', type: 'success' },
}

function typeMeta(section: GenerationSection) {
  return sectionTypeMeta[section.type] ?? { label: section.type, type: 'default' as const }
}
</script>

<template>
  <ol class="sec-progress">
    <li
      v-for="(s, idx) in sections"
      :key="s.id ?? idx"
      class="sec-progress__item"
      :class="{ 'sec-progress__item--active': s.status === SectionStatus.Generating }"
    >
      <div class="sec-progress__body">
        <div class="sec-progress__row">
          <span class="sec-progress__index">{{ idx + 1 }}</span>
          <span class="sec-progress__title">{{ s.title }}</span>
          <n-tag size="small" :bordered="false" :type="typeMeta(s).type">
            {{ typeMeta(s).label }}
          </n-tag>
        </div>
        <p v-if="s.knowledge_points?.length" class="sec-progress__points">
          {{ s.knowledge_points.join(' · ') }}
        </p>
      </div>

      <div class="sec-progress__status">
        <template v-if="s.status === SectionStatus.Generating">
          <n-spin size="small" />
          <n-tag size="small" :bordered="false" type="primary">生成中</n-tag>
        </template>
        <template v-else-if="s.status === SectionStatus.Done">
          <n-icon class="sec-progress__done-icon"><CheckmarkCircleOutline /></n-icon>
          <n-tag size="small" :bordered="false" type="success">已完成</n-tag>
        </template>
        <template v-else-if="s.status === SectionStatus.Failed">
          <n-icon class="sec-progress__failed-icon"><CloseCircleOutline /></n-icon>
          <n-tag size="small" :bordered="false" type="error">生成失败</n-tag>
          <n-button size="tiny" type="primary" ghost @click="emit('retry', s.id)">重试</n-button>
        </template>
        <template v-else>
          <n-icon class="sec-progress__clock-icon"><SyncOutline /></n-icon>
          <n-tag size="small" :bordered="false">待生成</n-tag>
        </template>
      </div>
    </li>
  </ol>
</template>

<style scoped>
.sec-progress {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sec-progress__item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 8px;
  background: var(--app-card-bg, #ffffff);
  transition: border-color 0.2s ease;
}

.sec-progress__item--active {
  border-color: var(--app-primary, #14b8a6);
  box-shadow: 0 0 0 1px var(--app-primary, #14b8a6) inset;
}

.sec-progress__body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.sec-progress__row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.sec-progress__index {
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

.sec-progress__title {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sec-progress__points {
  margin: 0;
  padding-left: 32px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--app-text-2, #64748b);
  word-break: break-word;
}

.sec-progress__status {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.sec-progress__done-icon {
  font-size: 18px;
  color: var(--app-success, #18a058);
}

.sec-progress__failed-icon {
  font-size: 18px;
  color: var(--app-error, #d03050);
}

.sec-progress__clock-icon {
  font-size: 18px;
  color: var(--app-text-3, #94a3b8);
}
</style>
