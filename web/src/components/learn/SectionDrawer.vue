<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NDrawer, NDrawerContent, NIcon } from 'naive-ui'
import { ListOutline } from '@vicons/ionicons5'

import type { SectionTypeValue } from '@/api/learn'
import { useLearnStore } from '@/stores/learn'

const store = useLearnStore()

const open = ref(false)

const typeLabel: Record<SectionTypeValue, string> = {
  slide: '讲解',
  quiz: '测验',
  demo_3d: '3D 演示',
  demo_function: '函数演示',
  demo_basic: '基础演示',
}

const typeHint: Record<SectionTypeValue, string> = {
  slide: '由老师讲解的图文页',
  quiz: '提交后展示答案与解析',
  demo_3d: '可交互的 3D 场景演示',
  demo_function: '可交互的函数/图表演示',
  demo_basic: '可运行并编辑的基础演示',
}

const entries = computed(() =>
  store.sections.map((s, i) => ({
    index: i,
    title: s.title,
    type: s.type,
    label: typeLabel[s.type],
    hint: typeHint[s.type],
    active: i === store.currentIndex,
  })),
)
</script>

<template>
  <div class="section-drawer">
    <n-button quaternary @click="open = true">
      <template #icon>
        <n-icon><ListOutline /></n-icon>
      </template>
      大纲
    </n-button>

    <n-drawer
      :show="open"
      placement="left"
      :width="320"
      @update:show="(v) => (open = v)"
    >
      <n-drawer-content closable>
        <template #header>
          <div class="section-drawer__head">
            <span class="section-drawer__title">课程大纲</span>
            <span v-if="store.detail" class="section-drawer__sub">
              {{ store.detail.course.title }}
            </span>
          </div>
        </template>

        <nav class="section-drawer__list" aria-label="课程大纲">
          <button
            v-for="e in entries"
            :key="e.index"
            type="button"
            class="section-drawer__item"
            :class="{ 'is-active': e.active }"
            :aria-current="e.active ? 'step' : undefined"
            :title="`${e.index + 1}. ${e.title}（${e.label}：${e.hint}）`"
            @click="store.goTo(e.index)"
          >
            <span class="section-drawer__seq">{{ e.index + 1 }}</span>
            <span class="section-drawer__name">{{ e.title }}</span>
            <span class="section-drawer__badge" :data-type="e.type">{{ e.label }}</span>
          </button>
        </nav>
      </n-drawer-content>
    </n-drawer>
  </div>
</template>

<style scoped>
.section-drawer__head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.section-drawer__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}
.section-drawer__sub {
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}

.section-drawer__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.section-drawer__item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-2, #64748b);
  font: inherit;
  font-size: 14px;
  text-align: left;
  cursor: pointer;
  transition:
    background 0.15s,
    color 0.15s,
    border-color 0.15s;
}
.section-drawer__item:hover {
  background: var(--app-divider, #e2e8f0);
}
.section-drawer__item.is-active {
  border-color: var(--app-primary, #14b8a6);
  background: var(--app-primary, #14b8a6);
  color: #fff;
}
.section-drawer__item.is-active .section-drawer__badge {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.6);
}

.section-drawer__seq {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--app-divider, #e2e8f0);
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-2, #64748b);
}
.section-drawer__item.is-active .section-drawer__seq {
  background: rgba(255, 255, 255, 0.25);
  color: #fff;
}

.section-drawer__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.section-drawer__badge {
  flex: none;
  padding: 0 6px;
  font-size: 11px;
  line-height: 1.5;
  border: 1px solid currentColor;
  border-radius: 999px;
}
.section-drawer__badge[data-type='quiz'] {
  color: #b45309;
}
.section-drawer__badge[data-type^='demo_'] {
  color: #0d9488;
}
.section-drawer__item.is-active .section-drawer__badge[data-type='quiz'],
.section-drawer__item.is-active .section-drawer__badge[data-type^='demo_'] {
  color: #fff;
}
</style>
