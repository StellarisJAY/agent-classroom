<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NEmpty, NSpin } from 'naive-ui'

import type { OutlineSection } from '@/api/course'
import OutlineList from '@/components/generate/OutlineList.vue'
import { useGenerationStore } from '@/stores/generation'

const route = useRoute()
const generationStore = useGenerationStore()

const courseId = computed(() => String(route.params.courseId))

/** 生成完成后的可编辑本地副本（调整位置/删除暂不落库，仅前端效果）。 */
const editable = ref<OutlineSection[]>([])

watch(
  () => generationStore.phase,
  (phase) => {
    if (phase === 'generated') {
      editable.value = generationStore.sections.map((s) => ({ ...s, knowledge_points: [...s.knowledge_points] }))
    }
  },
  { immediate: true },
)

function reorderSection(from: number, to: number) {
  const list = editable.value
  if (from === to || from < 0 || to < 0 || from >= list.length || to >= list.length) return
  const [item] = list.splice(from, 1)
  list.splice(to, 0, item!)
}

function retry() {
  generationStore.generate(courseId.value)
}

onMounted(() => {
  generationStore.initOrGenerate(courseId.value)
})
</script>

<template>
  <div class="generate-view">
    <!-- 生成中 -->
    <template v-if="generationStore.phase === 'generating'">
      <div class="generate-view__head">
        <n-spin size="small" />
        <span class="generate-view__status">正在生成大纲…</span>
      </div>

      <OutlineList
        v-if="generationStore.sections.length"
        :sections="generationStore.sections"
        readonly
      />
      <div v-else class="generate-view__empty">
        <n-spin size="large" />
      </div>
    </template>

    <!-- 生成完成 -->
    <div v-else-if="generationStore.phase === 'generated'" class="generate-view__done">
      <h2 class="generate-view__done-title">
        {{ generationStore.title || '大纲已生成' }}
      </h2>
      <OutlineList
        v-if="editable.length"
        :sections="editable"
        @move="reorderSection"
      />
      <p class="generate-view__note">大纲已生成。拖拽可调整环节顺序（暂未保存）。</p>
    </div>

    <!-- 错误/未开始 -->
    <div v-else class="generate-view__error">
      <n-empty v-if="generationStore.error" :description="generationStore.error">
        <template #extra>
          <n-button type="primary" @click="retry">重新生成</n-button>
        </template>
      </n-empty>
    </div>
  </div>
</template>

<style scoped>
.generate-view {
  width: 100%;
  max-width: 680px;
  margin: 0 auto;
  padding: 24px 20px;
}

.generate-view__head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.generate-view__status {
  font-size: 15px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}

.generate-view__empty {
  display: flex;
  justify-content: center;
  padding: 48px 0;
}

.generate-view__done-title {
  margin: 0 0 16px;
  font-size: 20px;
  font-weight: 700;
  color: var(--app-text-1, #0f172a);
}

.generate-view__note {
  margin-top: 16px;
  font-size: 13px;
  color: var(--app-text-2, #64748b);
  text-align: center;
}

.generate-view__error {
  display: flex;
  justify-content: center;
  padding: 40px 0;
}
</style>
