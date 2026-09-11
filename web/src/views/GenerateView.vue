<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NEmpty,
  NInput,
  NModal,
  NSpin,
  useMessage,
} from 'naive-ui'

import type { OutlineSection, OutlineVersionView } from '@/api/course'
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

// sections 每次都是整体替换赋值，按引用监听即可；回退场景 phase 不变、仅 sections 变化
watch(
  [() => generationStore.phase, () => generationStore.sections],
  ([phase, secs]) => {
    if (phase === 'generated') {
      editable.value = secs.map((s) => ({
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

/** 内联编辑环节内容描述（随确认提交，作为内容生成的固化要求）。 */
function updateDescription(idx: number, content: string) {
  const target = editable.value[idx]
  if (target) target.description = content
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
  openRegen(false)
}

/** 重新生成弹窗：required=true 时修改意见必填（确认步骤），false 时选填（失败重试）。 */
const regenModal = ref(false)
const regenRequired = ref(false)
const regenFeedback = ref('')
const regenBusy = ref(false)

function openRegen(required: boolean) {
  regenRequired.value = required
  regenFeedback.value = ''
  regenModal.value = true
}

async function confirmRegen() {
  if (regenRequired.value && !regenFeedback.value.trim()) {
    message.warning('请输入修改意见')
    return
  }
  regenBusy.value = true
  regenModal.value = false
  await generationStore.regenerate(courseId.value, regenFeedback.value.trim())
  regenBusy.value = false
}

/** 历史版本弹窗 */
const versionsModal = ref(false)
const versions = ref<OutlineVersionView[]>([])
const versionsLoading = ref(false)
const reverting = ref(false)

async function openVersions() {
  versionsLoading.value = true
  versionsModal.value = true
  try {
    versions.value = await generationStore.listVersions(courseId.value)
  } catch {
    versions.value = []
    message.error('加载历史版本失败')
  } finally {
    versionsLoading.value = false
  }
}

async function handleRevert(version: number) {
  reverting.value = true
  try {
    await generationStore.revert(courseId.value, version)
    versionsModal.value = false
    message.success(`已回退到第 ${version} 版`)
  } catch (e) {
    message.error(e instanceof Error ? e.message : '回退失败')
  } finally {
    reverting.value = false
  }
}

function formatTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 内容生成中断/失败后恢复后台续跑，并重新进入进度轮询。 */
function retryContent() {
  generationStore.resumeGeneration(courseId.value)
}

function goLearn() {
  router.push(`/course/${courseId.value}/learn`)
}

onMounted(async () => {
  await generationStore.initOrGenerate(courseId.value)
  await generationStore.resumeContentIfNeeded(courseId.value)
})

onBeforeUnmount(() => {
  generationStore.stopPolling()
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
      <OutlineList
        v-if="editable.length"
        :sections="editable"
        @move="reorderSection"
        @update-description="updateDescription"
      />
      <n-empty v-else description="大纲为空" />

      <div class="generate-view__confirm-actions">
        <n-button type="primary" size="large" :loading="confirming" :disabled="!editable.length" @click="handleConfirm">
          确认并开始生成
        </n-button>
        <n-button size="large" @click="openRegen(true)">重新生成</n-button>
        <n-button size="large" @click="openVersions">历史版本</n-button>
      </div>
    </div>

    <!-- 重新生成弹窗 -->
    <n-modal v-model:show="regenModal" preset="card" title="重新生成大纲" style="width: 440px">
      <p class="generate-view__modal-hint">
        输入修改意见，AI 将参考当前大纲与最初要求进行调整（意见为{{ regenRequired ? '必填' : '选填' }}，不填则重新生成）。
      </p>
      <n-input
        v-model:value="regenFeedback"
        type="textarea"
        :rows="4"
        placeholder="例如：增加一个实操环节，第 2 节标题更通俗…"
        :disabled="regenBusy"
      />
      <template #footer>
        <div class="generate-view__modal-actions">
          <n-button @click="regenModal = false">取消</n-button>
          <n-button type="primary" :loading="regenBusy" @click="confirmRegen">确认重新生成</n-button>
        </div>
      </template>
    </n-modal>

    <!-- 历史版本弹窗 -->
    <n-modal v-model:show="versionsModal" preset="card" title="大纲历史版本" style="width: 520px">
      <n-spin :show="versionsLoading">
        <div v-if="!versionsLoading && versions.length" class="generate-view__versions">
          <button
            v-for="v in versions"
            :key="v.version"
            type="button"
            class="generate-view__version"
            :disabled="v.current || reverting"
            @click="handleRevert(v.version)"
          >
            <div class="generate-view__version-line">
              <span class="generate-view__version-tag">第 {{ v.version }} 版</span>
              <span v-if="v.current" class="generate-view__version-current">当前</span>
              <span class="generate-view__version-time">{{ formatTime(v.created_at) }}</span>
            </div>
            <div v-if="v.feedback" class="generate-view__version-feedback">{{ v.feedback }}</div>
          </button>
        </div>
        <n-empty v-else-if="!versionsLoading" description="暂无历史版本" />
      </n-spin>
      <template #footer>
        <div class="generate-view__modal-actions">
          <n-button @click="versionsModal = false">关闭</n-button>
        </div>
      </template>
    </n-modal>
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
  gap: 12px;
}

.generate-view__modal-hint {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--app-text-2, #64748b);
}

.generate-view__modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.generate-view__versions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 360px;
  overflow: auto;
}

.generate-view__version {
  display: block;
  width: 100%;
  text-align: left;
  padding: 10px 12px;
  border: 1px solid var(--app-border, #e2e8f0);
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  transition: border-color 0.2s;
}

.generate-view__version:hover:not(:disabled) {
  border-color: var(--app-primary, #2563eb);
}

.generate-view__version:disabled {
  cursor: default;
  opacity: 0.9;
}

.generate-view__version-line {
  display: flex;
  align-items: center;
  gap: 8px;
}

.generate-view__version-tag {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}

.generate-view__version-current {
  font-size: 12px;
  color: var(--app-primary, #2563eb);
  border: 1px solid currentColor;
  border-radius: 999px;
  padding: 0 6px;
}

.generate-view__version-time {
  margin-left: auto;
  font-size: 12px;
  color: var(--app-text-3, #94a3b8);
}

.generate-view__version-feedback {
  margin-top: 4px;
  font-size: 12px;
  color: var(--app-text-2, #64748b);
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
