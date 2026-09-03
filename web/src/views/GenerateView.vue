<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NEmpty, NSpin, useMessage } from 'naive-ui'

import type { OutlineSection } from '@/api/course'
import OutlineList from '@/components/generate/OutlineList.vue'
import SectionProgressList from '@/components/generate/SectionProgressList.vue'
import { useGenerationStore } from '@/stores/generation'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const generationStore = useGenerationStore()

const courseId = computed(() => String(route.params.courseId))

/** 大纲生成完成后的可编辑本地副本（调序/删除仅前端效果，确认时提交给后端）。 */
const editable = ref<OutlineSection[]>([])

watch(
  () => generationStore.phase,
  (phase) => {
    if (phase === 'generated') {
      editable.value = generationStore.sections.map((s) => ({
        ...s,
        knowledge_points: [...s.knowledge_points],
      }))
    }
  },
  { immediate: true },
)

/** 大纲已就绪、尚未确认 → 展示确认按钮 */
const showConfirm = computed(
  () => generationStore.phase === 'generated' && generationStore.contentPhase === 'idle',
)
const confirming = ref(false)

function reorderSection(from: number, to: number) {
  const list = editable.value
  if (from === to || from < 0 || to < 0 || from >= list.length || to >= list.length) return
  const [item] = list.splice(from, 1)
  list.splice(to, 0, item!)
}

async function handleConfirm() {
  if (!editable.value.length) {
    message.warning('大纲为空，无法生成课程内容')
    return
  }
  confirming.value = true
  await generationStore.startContent(courseId.value, editable.value.map((s) => ({ ...s })))
  confirming.value = false
}

function retry() {
  generationStore.generate(courseId.value)
}

/** 内容生成失败后重连订阅（服务端会续跑剩余环节）。 */
function retryContent() {
  generationStore.resumeContentIfNeeded(courseId.value)
}

function goLearn() {
  router.push(`/course/${courseId.value}/learn`)
}

onMounted(async () => {
  await generationStore.initOrGenerate(courseId.value)
  await generationStore.resumeContentIfNeeded(courseId.value)
})
</script>

<template>
  <div class="generate-view">
    <!-- 大纲生成中 -->
    <template v-if="generationStore.phase === 'generating'">
      <div class="generate-view__head">
        <n-spin size="small" />
        <span class="generate-view__status">正在生成大纲…</span>
      </div>
      <OutlineList v-if="generationStore.sections.length" :sections="generationStore.sections" readonly />
      <div v-else class="generate-view__empty">
        <n-spin size="large" />
      </div>
    </template>

    <!-- 大纲生成失败 -->
    <div v-else-if="generationStore.error && generationStore.contentPhase === 'idle'" class="generate-view__error">
      <n-empty :description="generationStore.error">
        <template #extra>
          <n-button type="primary" @click="retry">重新生成大纲</n-button>
        </template>
      </n-empty>
    </div>

    <!-- 内容生成中 -->
    <template v-else-if="generationStore.contentPhase === 'generating'">
      <div class="generate-view__head">
        <n-spin size="small" />
        <span class="generate-view__status">正在生成课程内容…</span>
      </div>
      <SectionProgressList v-if="generationStore.progress.length" :sections="generationStore.progress" />
      <div v-else class="generate-view__empty">
        <n-spin size="large" />
      </div>
      <p class="generate-view__note">内容在后台按顺序逐环节生成，可离开本页稍后回来查看进度。</p>
    </template>

    <!-- 内容生成失败 -->
    <div v-else-if="generationStore.contentPhase === 'error'" class="generate-view__error">
      <n-empty :description="generationStore.contentError || '课程内容生成失败'">
        <template #extra>
          <n-button type="primary" @click="retryContent">重试继续生成</n-button>
        </template>
      </n-empty>
    </div>

    <!-- 内容生成完成 -->
    <div v-else-if="generationStore.contentPhase === 'done'" class="generate-view__done">
      <h2 class="generate-view__done-title">{{ generationStore.title || '课程内容已生成' }}</h2>
      <SectionProgressList v-if="generationStore.progress.length" :sections="generationStore.progress" />
      <p class="generate-view__note">全部环节已生成完成。</p>
      <div class="generate-view__done-actions">
        <n-button type="primary" @click="goLearn">进入学习</n-button>
        <n-button @click="router.push('/')">返回课程库</n-button>
      </div>
    </div>

    <!-- 大纲确认（Step 2） -->
    <div v-else-if="showConfirm" class="generate-view__confirm">
      <h2 class="generate-view__confirm-title">
        {{ generationStore.title || '大纲已生成' }}
      </h2>
      <p class="generate-view__confirm-hint">
        拖拽可调整环节顺序、删除不需要的环节，确认后按顺序生成课程内容。
      </p>
      <OutlineList v-if="editable.length" :sections="editable" @move="reorderSection" />
      <n-empty v-else description="大纲为空" />

      <div class="generate-view__confirm-actions">
        <n-button type="primary" size="large" :loading="confirming" :disabled="!editable.length" @click="handleConfirm">
          确认并开始生成
        </n-button>
      </div>
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

.generate-view__error {
  display: flex;
  justify-content: center;
  padding: 40px 0;
}

.generate-view__done-title,
.generate-view__confirm-title {
  margin: 0 0 8px;
  font-size: 20px;
  font-weight: 700;
  color: var(--app-text-1, #0f172a);
}

.generate-view__confirm-hint {
  margin: 0 0 16px;
  font-size: 13px;
  color: var(--app-text-2, #64748b);
}

.generate-view__confirm-actions {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}

.generate-view__done-actions {
  margin-top: 16px;
  display: flex;
  justify-content: center;
  gap: 12px;
}

.generate-view__note {
  margin-top: 16px;
  font-size: 13px;
  color: var(--app-text-2, #64748b);
  text-align: center;
}
</style>
