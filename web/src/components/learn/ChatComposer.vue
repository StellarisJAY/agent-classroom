<script setup lang="ts">
// 输入区：Drawer 壳与讨论侧板共用。Enter 发送、Shift+Enter 换行。
import { ref } from 'vue'
import { NButton, NIcon, NInput, NTooltip } from 'naive-ui'
import { PaperPlaneOutline } from '@vicons/ionicons5'

const draft = defineModel<string>({ default: '' })
const disabled = defineModel<boolean>('disabled', { default: false })

const inputRef = ref<InstanceType<typeof NInput> | null>(null)

const emit = defineEmits<{ send: [text: string] }>()

function submitUserText() {
  const text = draft.value.trim()
  if (!text || disabled.value) return
  draft.value = ''
  emit('send', text)
}

defineExpose({ focus: () => inputRef.value?.focus() })
</script>

<template>
  <div class="chat-composer">
    <n-input
      ref="inputRef"
      v-model:value="draft"
      type="textarea"
      :autosize="{ minRows: 1, maxRows: 4 }"
      placeholder="输入你的问题，Enter 发送，Shift+Enter 换行"
      :disabled="disabled"
      @keydown.enter.exact.prevent="submitUserText"
    />
    <n-tooltip placement="top">
      <template #trigger>
        <n-button
          circle
          type="primary"
          :disabled="!draft.trim() || disabled"
          :loading="disabled"
          aria-label="发送"
          @click="submitUserText"
        >
          <template #icon>
            <n-icon><PaperPlaneOutline /></n-icon>
          </template>
        </n-button>
      </template>
      发送
    </n-tooltip>
  </div>
</template>

<style scoped>
.chat-composer {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  padding: 12px 16px;
  padding-bottom: calc(12px + env(safe-area-inset-bottom, 0px));
  border-top: 1px solid var(--app-divider, #e2e8f0);
}
.chat-composer :deep(.n-input) {
  flex: 1;
}
</style>
