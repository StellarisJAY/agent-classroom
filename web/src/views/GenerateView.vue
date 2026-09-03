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

function moveSection(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= editable.value.length) return
  const [item] = editable.value.splice(index, 1)
  editable.value.splice(target, 0, item!)
}

function removeSection(index: number) {
  editable.value.splice(index, 1)
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
        reorderable
        @move-up="(i) => moveSection(i, -1)"
        @move-down="(i) => moveSection(i, 1)"
        @remove="removeSection"
      />
      <p class="generate-view__note">大纲已生成。调整位置或删除环节仅影响当前预览，暂未保存。</p>
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
