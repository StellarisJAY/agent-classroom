<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NInput, NUpload, useMessage } from 'naive-ui'
import type { UploadFileInfo } from 'naive-ui'
import { RocketOutline } from '@vicons/ionicons5'

import { createCourse } from '@/api/course'

const router = useRouter()
const message = useMessage()

const prompt = ref('')
const files = ref<UploadFileInfo[]>([])
const submitting = ref(false)

const ALLOWED_EXTS = ['.txt', '.md', '.markdown']

function isAllowed(name: string): boolean {
  const lower = name.toLowerCase()
  return ALLOWED_EXTS.some((ext) => lower.endsWith(ext))
}

function handleFileChange({ file, fileList }: { file: UploadFileInfo; fileList: UploadFileInfo[] }) {
  const f = file.file
  if (f && !isAllowed(f.name)) {
    message.error('参考文档仅支持 txt / md 格式')
    // 移除非法项
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
    const course = await createCourse(prompt.value.trim(), rawFiles)
    message.success('课程已创建，正在生成大纲…')
    router.push(`/preview/${course.id}`)
  } catch (e) {
    message.error(e instanceof Error ? e.message : '创建课程失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="create-view">
    <div class="create-view__card">
      <header class="create-view__head">
        <h1 class="create-view__title">AGENT-C</h1>
        <p class="create-view__subtitle">描述你想学习的内容，AI 将生成可互动课程</p>
      </header>

      <div class="create-view__prompt">
        <label class="create-view__label" for="course-prompt">课程内容要求</label>
        <n-input
          id="course-prompt"
          v-model:value="prompt"
          type="textarea"
          placeholder="例如：系统讲解一维数组的声明、初始化、遍历与常见操作，帮助零基础学习者建立编程直觉。"
          :autosize="{ minRows: 4, maxRows: 8 }"
          :disabled="submitting"
          maxlength="2000"
          show-count
        />
      </div>

      <div class="create-view__upload">
        <span class="create-view__label">参考文档（txt / md，可选）</span>
        <n-upload
          accept=".txt,.md,.markdown,text/plain,text/markdown"
          :default-upload="false"
          multiple
          :max="8"
          :file-list="files"
          @update:file-list="files = $event"
          @change="handleFileChange"
        >
          <n-button :disabled="submitting">选择文档</n-button>
        </n-upload>
      </div>

      <div class="create-view__actions">
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
    </div>
  </div>
</template>

<style scoped>
.create-view {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
  overflow-y: auto;
}

.create-view__card {
  width: 100%;
  max-width: 560px;
  padding: 28px;
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 12px;
  background: var(--app-card-bg, #ffffff);
}

.create-view__head {
  text-align: center;
  margin-bottom: 20px;
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

.create-view__label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}

.create-view__prompt,
.create-view__upload {
  margin-bottom: 18px;
}

.create-view__actions {
  display: flex;
  justify-content: flex-end;
}

.create-view__submit {
  min-width: 160px;
}
</style>
