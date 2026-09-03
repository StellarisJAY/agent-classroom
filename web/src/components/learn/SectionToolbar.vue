<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NIcon } from 'naive-ui'
import { ChevronBack, ChevronForwardOutline } from '@vicons/ionicons5'

import type { SectionTypeValue } from '@/api/learn'
import { useLearnStore } from '@/stores/learn'

const store = useLearnStore()

const typeLabel: Record<SectionTypeValue, string> = {
  slide: '讲解',
  quiz: '测验',
  demo: '演示',
}

const typeHint: Record<SectionTypeValue, string> = {
  slide: '由老师讲解的图文页',
  quiz: '提交后展示答案与解析',
  demo: '可交互并编辑的演示',
}

const pills = computed(() =>
  store.sections.map((s, i) => ({
    index: i,
    title: s.title,
    type: s.type,
    label: typeLabel[s.type],
    active: i === store.currentIndex,
  })),
)
</script>

<template>
  <div class="section-toolbar" role="toolbar" aria-label="环节切换">
    <n-button
      quaternary
      circle
      aria-label="上一节"
      class="section-toolbar__nav"
      :disabled="!store.hasPrevSection"
      @click="store.prevSection()"
    >
      <template #icon>
        <n-icon><ChevronBack /></n-icon>
      </template>
    </n-button>

    <div class="section-toolbar__list">
      <button
        v-for="p in pills"
        :key="p.index"
        type="button"
        class="section-toolbar__item"
        :class="{ 'is-active': p.active }"
        :aria-label="`${p.index + 1}. ${p.title}（${p.label}）`"
        :aria-current="p.active ? 'step' : undefined"
        :title="`${p.title}（${p.label}：${typeHint[p.type]}）`"
        @click="store.goTo(p.index)"
      >
        <span class="section-toolbar__seq">{{ p.index + 1 }}</span>
        <span class="section-toolbar__name">{{ p.title }}</span>
        <span class="section-toolbar__badge" :data-type="p.type">{{ p.label }}</span>
      </button>
    </div>

    <n-button
      quaternary
      circle
      aria-label="下一节"
      class="section-toolbar__nav"
      :disabled="!store.hasNextSection"
      @click="store.nextSection()"
    >
      <template #icon>
        <n-icon><ChevronForwardOutline /></n-icon>
      </template>
    </n-button>
  </div>
</template>

<style scoped>
.section-toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
}

.section-toolbar__list {
  display: flex;
  gap: 8px;
  align-items: stretch;
  overflow-x: auto;
  padding: 2px;
  scrollbar-width: none;
}
.section-toolbar__list::-webkit-scrollbar {
  display: none;
}

.section-toolbar__item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 220px;
  padding: 5px 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-2, #64748b);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
  transition:
    background 0.15s,
    color 0.15s,
    border-color 0.15s;
}
.section-toolbar__item:hover {
  background: var(--app-divider, #e2e8f0);
}
.section-toolbar__item.is-active {
  border-color: var(--app-primary, #14b8a6);
  background: var(--app-primary, #14b8a6);
  color: #fff;
}
.section-toolbar__item.is-active .section-toolbar__badge {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.6);
}

.section-toolbar__seq {
  font-weight: 600;
  opacity: 0.75;
}

.section-toolbar__name {
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 500;
}

.section-toolbar__badge {
  flex: none;
  padding: 0 6px;
  font-size: 11px;
  line-height: 1.5;
  border: 1px solid currentColor;
  border-radius: 999px;
}
.section-toolbar__badge[data-type='quiz'] {
  color: #b45309;
}
.section-toolbar__badge[data-type='demo'] {
  color: #0d9488;
}
.section-toolbar__item.is-active .section-toolbar__badge[data-type='quiz'],
.section-toolbar__item.is-active .section-toolbar__badge[data-type='demo'] {
  color: #fff;
}
</style>
