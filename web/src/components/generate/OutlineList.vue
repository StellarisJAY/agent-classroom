<script setup lang="ts">
import { ref } from 'vue'
import type { TagProps } from 'naive-ui'
import { NButton, NIcon, NInput, NTag } from 'naive-ui'
import { MenuOutline, PencilOutline } from '@vicons/ionicons5'

import { SectionType, type OutlineSection } from '@/api/course'

const props = defineProps<{
  sections: OutlineSection[]
  /** 只读：不可拖拽换序（如生成中轮询阶段） */
  readonly?: boolean
}>()

const emit = defineEmits<{
  /** 从 from 移到 to */
  move: [from: number, to: number]
  /** 确认阶段内联编辑环节内容描述 */
  'update-description': [index: number, content: string]
}>()

/** 描述内联编辑：正在编辑的索引与草稿内容（一次一个）。 */
const descEditing = ref<number | null>(null)
const descDraft = ref('')

function startDescEdit(idx: number) {
  descEditing.value = idx
  descDraft.value = props.sections[idx]?.description ?? ''
}

function commitDescEdit() {
  if (descEditing.value !== null) {
    emit('update-description', descEditing.value, descDraft.value.trim())
  }
  descEditing.value = null
}

function cancelDescEdit() {
  descEditing.value = null
}

/** 当前正在拖拽的起始索引 */
const dragIndex = ref<number | null>(null)
/** 当前高亮的放置目标索引 */
const overIndex = ref<number | null>(null)

const sectionTypeMeta: Record<string, { label: string; type: TagProps['type'] }> = {
  [SectionType.Slide]: { label: '讲解', type: 'info' },
  [SectionType.Quiz]: { label: '测试', type: 'warning' },
  [SectionType.Demo3D]: { label: '3D 演示', type: 'success' },
  [SectionType.DemoFunction]: { label: '函数演示', type: 'success' },
  [SectionType.DemoBasic]: { label: '基础演示', type: 'success' },
}

function typeMeta(section: OutlineSection) {
  return sectionTypeMeta[section.type] ?? { label: section.type, type: 'default' }
}

function onDragStart(idx: number, event: DragEvent) {
  if (props.readonly) return
  dragIndex.value = idx
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(idx))
  }
}

function onDragOver(idx: number, event: DragEvent) {
  if (props.readonly || dragIndex.value === null) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
  overIndex.value = idx
}

function onDrop(idx: number) {
  if (dragIndex.value !== null && dragIndex.value !== idx) {
    emit('move', dragIndex.value, idx)
  }
  dragIndex.value = null
  overIndex.value = null
}

function onDragEnd() {
  dragIndex.value = null
  overIndex.value = null
}
</script>

<template>
  <ol class="outline-list">
    <li
      v-for="(s, idx) in props.sections"
      :key="idx"
      class="outline-list__item"
      :class="{
        'outline-list__item--dragging': dragIndex === idx,
        'outline-list__item--over': overIndex === idx && dragIndex !== idx,
        'outline-list__item--draggable': !readonly,
      }"
      :draggable="!readonly"
      @dragstart="onDragStart(idx, $event)"
      @dragover="onDragOver(idx, $event)"
      @drop="onDrop(idx)"
      @dragend="onDragEnd"
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
        <p v-if="s.description && descEditing !== idx" class="outline-list__desc">
          {{ s.description }}
        </p>
        <div v-if="descEditing === idx" class="outline-list__desc-editor">
          <n-input
            v-model:value="descDraft"
            type="textarea"
            :rows="3"
            maxlength="600"
            show-count
            placeholder="描述该环节将生成的内容（讲解组织/题型题量/演示目标）…"
          />
          <div class="outline-list__desc-actions">
            <n-button size="tiny" @click="cancelDescEdit">取消</n-button>
            <n-button size="tiny" type="primary" @click="commitDescEdit">保存</n-button>
          </div>
        </div>
      </div>

      <div class="outline-list__actions">
        <n-button
          v-if="!readonly && descEditing !== idx"
          quaternary
          size="tiny"
          class="outline-list__edit"
          @click="startDescEdit(idx)"
        >
          <template #icon>
            <n-icon><PencilOutline /></n-icon>
          </template>
          编辑描述
        </n-button>
        <n-icon v-if="!readonly" class="outline-list__grip" title="拖拽调整顺序"><MenuOutline /></n-icon>
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
  transition: opacity 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

.outline-list__item--draggable {
  cursor: grab;
}

.outline-list__item--draggable:active {
  cursor: grabbing;
}

.outline-list__item--dragging {
  opacity: 0.45;
}

.outline-list__item--over {
  border-color: var(--app-primary, #14b8a6);
  box-shadow: 0 0 0 1px var(--app-primary, #14b8a6) inset;
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

.outline-list__desc {
  margin: 0;
  padding-left: 32px;
  font-size: 12px;
  line-height: 1.55;
  color: var(--app-text-3, #94a3b8);
  word-break: break-word;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.outline-list__desc-editor {
  padding-left: 32px;
}

.outline-list__desc-actions {
  margin-top: 4px;
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}

.outline-list__actions {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
}

.outline-list__edit {
  align-self: flex-start;
}

.outline-list__grip {
  font-size: 18px;
  pointer-events: none;
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
