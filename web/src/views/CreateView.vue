<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NIcon, NInput, NInputNumber, NSelect, NSlider, NTooltip, NUpload, useMessage } from 'naive-ui'
import type { SelectOption, UploadFileInfo } from 'naive-ui'
import { RocketOutline, AttachOutline } from '@vicons/ionicons5'

import { OutlineCount, Thinking, type ThinkingValue, createCourse } from '@/api/course'
import { useModelConfigStore } from '@/stores/model-config'

const router = useRouter()
const message = useMessage()
const modelConfigStore = useModelConfigStore()

const prompt = ref('')
const files = ref<UploadFileInfo[]>([])
const submitting = ref(false)

/** 大纲环节数量上限 */
const outlineCount = ref(OutlineCount.Default)

/** 选中的模型配置；空串表示使用默认模型（不随课程绑定专属配置） */
const modelConfigId = ref('')
const thinking = ref<ThinkingValue>(Thinking.Default)

const ALLOWED_EXTS = ['.txt', '.md', '.markdown']

const modelOptions = computed<SelectOption[]>(() => {
  const label = modelConfigStore.defaultConfig
    ? `默认 · ${modelConfigStore.defaultConfig.model}`
    : '服务端默认模型'
  const opts: SelectOption[] = [{ label, value: '' }]
  for (const c of modelConfigStore.configs) {
    opts.push({ label: `${c.model}（${c.provider}）`, value: c.id })
  }
  return opts
})

const thinkingOptions: SelectOption[] = [
  { label: '关闭思考', value: Thinking.Off },
  { label: '默认', value: Thinking.Default },
  { label: '最大化思考', value: Thinking.Max },
]

function isAllowed(name: string): boolean {
  const lower = name.toLowerCase()
  return ALLOWED_EXTS.some((ext) => lower.endsWith(ext))
}

function handleModelChange(value: string | number | null) {
  modelConfigId.value = value ? String(value) : ''
}

function handleFileChange({ file, fileList }: { file: UploadFileInfo; fileList: UploadFileInfo[] }) {
  const f = file.file
  if (f && !isAllowed(f.name)) {
    message.error('参考文档仅支持 txt / md 格式')
    files.value = fileList.filter((i) => i.id !== file.id)
    return
  }
  if (f && f.size > 10 * 1024 * 1024) {
    message.error('单个参考文档不能超过 10MB')
    files.value = fileList.filter((i) => i.id !== file.id)
    return
  }
  files.value = fileList
}

function removeFile(id: string) {
  files.value = files.value.filter((i) => i.id !== id)
}

async function handleSubmit() {
  if (!prompt.value.trim()) {
    message.warning('请描述你想学习的课程内容要求')
    return
  }
  const rawFiles = files.value
    .map((i) => i.file)
    .filter((f): f is File => Boolean(f))

  submitting.value = true
  try {
    const course = await createCourse(prompt.value.trim(), rawFiles, {
      modelConfigId: modelConfigId.value || undefined,
      thinking: thinking.value,
      outlineCount: outlineCount.value,
    })
    message.success('课程已创建，正在生成大纲…')
    router.push(`/preview/${course.id}`)
  } catch (e) {
    message.error(e instanceof Error ? e.message : '创建课程失败')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  modelConfigStore.ensureLoaded()
})
</script>

<template>
  <div class="create-view">
    <header class="create-view__head">
      <h1 class="create-view__title">AGENT-C</h1>
      <p class="create-view__subtitle">描述你想学习的内容，AI 将生成可互动课程</p>
    </header>

    <div class="create-view__composer">
      <n-input
        v-model:value="prompt"
        type="textarea"
        :bordered="false"
        class="create-view__textarea"
        placeholder="描述你想学习的课程内容，例如：系统讲解一维数组的声明、初始化、遍历与常见操作，帮助零基础学习者建立编程直觉。"
        :autosize="{ minRows: 5, maxRows: 12 }"
        :disabled="submitting"
        maxlength="2000"
      />

      <div class="create-view__files">
        <n-tag
          v-for="f in files"
          :key="f.id"
          size="small"
          closable
          :bordered="false"
          :disabled="submitting"
          @close="removeFile(f.id)"
        >
          {{ f.name }}
        </n-tag>
      </div>

      <div class="create-view__outline">
        <div class="create-view__outline-head">
          <span class="create-view__outline-label">课程环节数量</span>
          <n-input-number
            v-model:value="outlineCount"
            :min="OutlineCount.Min"
            :max="OutlineCount.Max"
            :disabled="submitting"
            size="small"
            class="create-view__outline-num"
          />
        </div>
        <n-slider
          v-model:value="outlineCount"
          :min="OutlineCount.Min"
          :max="OutlineCount.Max"
          :step="1"
          :disabled="submitting"
        />
        <p v-if="outlineCount > 15" class="create-view__outline-warn">
          环节较多（&gt;15）：生成时间会更长，且消耗的 Token 也会更多。
        </p>
        <p v-else class="create-view__outline-hint">
          最多约 {{ outlineCount }} 个环节，AI 按需拆解，不强制凑满。
        </p>
      </div>

      <div class="create-view__toolbar">
        <div class="create-view__tools">
          <n-select
            :value="modelConfigId"
            :options="modelOptions"
            size="small"
            class="create-view__tool create-view__tool--model"
            :disabled="submitting"
            placeholder="模型"
            @update:value="handleModelChange"
          />
          <n-select
            :value="thinking"
            :options="thinkingOptions"
            size="small"
            class="create-view__tool create-view__tool--thinking"
            :disabled="submitting"
            placeholder="思考"
            @update:value="thinking = $event as ThinkingValue"
          />
          <n-tooltip placement="top">
            <template #trigger>
              <n-upload
                accept=".txt,.md,.markdown,text/plain,text/markdown"
                :default-upload="false"
                multiple
                :max="8"
                :show-file-list="false"
                :file-list="files"
                :disabled="submitting"
                @update:file-list="files = $event"
                @change="handleFileChange"
              >
                <n-button quaternary circle :disabled="submitting" aria-label="上传参考文档">
                  <template #icon>
                    <n-icon size="18"><AttachOutline /></n-icon>
                  </template>
                </n-button>
              </n-upload>
            </template>
            上传参考文档（txt / md）
          </n-tooltip>
        </div>

        <n-button
          type="primary"
          size="large"
          :loading="submitting"
          class="create-view__submit"
          @click="handleSubmit"
        >
          <template #icon><RocketOutline /></template>
          生成课程
        </n-button>
      </div>

      <p v-if="!modelConfigStore.hasConfig" class="create-view__hint">
        尚未添加模型配置，将使用服务端默认模型
      </p>
    </div>
  </div>
</template>

<style scoped>
.create-view {
  width: 100%;
  max-width: 720px;
  margin: 0 auto;
  padding: 32px 20px;
  display: flex;
  flex-direction: column;
}

.create-view__head {
  text-align: center;
  margin-bottom: 24px;
}

.create-view__title {
  margin: 0 0 6px;
  font-size: 24px;
  font-weight: 700;
  background: linear-gradient(120deg, #14b8a6, #0d9488);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.create-view__subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--app-text-2, #64748b);
}

.create-view__composer {
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 16px;
  background: var(--app-card-bg, #ffffff);
  padding: 12px 12px 10px;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.create-view__composer:focus-within {
  border-color: var(--app-primary, #14b8a6);
  box-shadow: 0 0 0 1px var(--app-primary, #14b8a6);
}

.create-view__textarea :deep(.n-input-wrapper) {
  background: transparent;
}

.create-view__textarea :deep(.n-input__textarea) {
  font-size: 15px;
  line-height: 1.7;
  color: var(--app-text-1, #0f172a);
}

.create-view__files {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 4px 2px 10px;
}

.create-view__outline {
  padding: 4px 8px 8px;
}

.create-view__outline-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 4px;
}

.create-view__outline-label {
  font-size: 13px;
  color: var(--app-text-2, #64748b);
}

.create-view__outline-num {
  width: 84px;
}

.create-view__outline-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}

.create-view__outline-warn {
  margin: 4px 0 0;
  font-size: 12px;
  color: #b45309;
}

.create-view__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--app-divider, #e2e8f0);
  padding-top: 10px;
}

.create-view__tools {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.create-view__tool--model {
  width: 170px;
}

.create-view__tool--thinking {
  width: 120px;
}

.create-view__submit {
  flex-shrink: 0;
}

.create-view__hint {
  margin: 8px 2px 0;
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}
</style>
