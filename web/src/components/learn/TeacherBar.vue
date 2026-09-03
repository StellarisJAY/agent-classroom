<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { NButton, NIcon, NTooltip } from 'naive-ui'
import { ChatbubbleEllipsesOutline, VolumeHighOutline } from '@vicons/ionicons5'

import { useConversationStore } from '@/stores/conversation'
import { useLearnStore } from '@/stores/learn'

const store = useLearnStore()
const chat = useConversationStore()

// 常驻旁白：slide 显示当前步骤讲解；quiz/demo 显示引导文案（quiz 不泄露答案）
const helperText: Record<string, string> = {
  quiz: '认真作答后再提交，我会在不透露答案的前提下引导你思考。',
  demo: '可以运行并编辑这段演示代码，动手体验更直观。',
}

const narration = computed<string>(() => {
  const s = store.currentSection
  if (!s) return ''
  if (s.type === 'slide') return store.currentStep?.text ?? ''
  return helperText[s.type] ?? ''
})

// 逐字显示（typewriter）
const shown = ref('')
let timer: ReturnType<typeof setInterval> | null = null

function startTyping() {
  if (timer) clearInterval(timer)
  const text = narration.value
  if (!text) {
    shown.value = ''
    return
  }
  if (!store.isSlide) {
    shown.value = text
    return
  }
  shown.value = ''
  let i = 0
  timer = setInterval(() => {
    i += 1
    shown.value = text.slice(0, i)
    if (i >= text.length && timer) clearInterval(timer)
  }, 26)
}

watch([narration, () => store.currentSection?.id], startTyping, { immediate: true })

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="teacher-bar" role="region" aria-label="智能老师旁白">
    <div class="teacher-bar__avatar" aria-hidden="true">师</div>

    <div class="teacher-bar__narration">
      <span class="teacher-bar__label">讲解</span>
      <p class="teacher-bar__text">
        {{ shown }}<span v-if="shown.length < narration.length" class="teacher-bar__caret" />
      </p>
    </div>

    <div class="teacher-bar__actions">
      <n-tooltip v-if="store.isSlide && narration" placement="top">
        <template #trigger>
          <n-button
            quaternary
            circle
            aria-label="语音朗读（待开放）"
            aria-disabled="true"
            @click="() => {}"
          >
            <template #icon>
              <n-icon><VolumeHighOutline /></n-icon>
            </template>
          </n-button>
        </template>
        语音朗读将在后续版本提供（本期占位）
      </n-tooltip>

      <n-button type="primary" @click="chat.openPanel()">
        <template #icon>
          <n-icon><ChatbubbleEllipsesOutline /></n-icon>
        </template>
        向老师提问
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.teacher-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-top: 1px solid var(--app-divider, #e2e8f0);
  background: var(--app-header-bg, #ffffff);
}

.teacher-bar__avatar {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  color: #fff;
  background: linear-gradient(135deg, #14b8a6, #0d9488);
  font-size: 15px;
  font-weight: 600;
}

.teacher-bar__narration {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
.teacher-bar__label {
  flex: none;
  margin-top: 2px;
  padding: 0 6px;
  font-size: 11px;
  border-radius: 999px;
  color: var(--app-primary, #14b8a6);
  border: 1px solid currentColor;
}
.teacher-bar__text {
  flex: 1;
  min-width: 0;
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--app-text-1, #0f172a);
  white-space: pre-wrap;
}
.teacher-bar__caret {
  display: inline-block;
  width: 2px;
  height: 1em;
  vertical-align: text-bottom;
  margin-left: 1px;
  background: var(--app-primary, #14b8a6);
  animation: blink 0.9s steps(1) infinite;
}
@keyframes blink {
  50% {
    opacity: 0;
  }
}

.teacher-bar__actions {
  flex: none;
  display: flex;
  align-items: center;
  gap: 4px;
}
.is-muted {
  color: var(--app-text-2, #64748b);
}
</style>
