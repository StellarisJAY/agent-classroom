<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { NButton, NIcon, NTooltip } from 'naive-ui'
import {
  ChatbubbleEllipsesOutline,
  ChevronDownOutline,
  ChevronUpOutline,
  VolumeHighOutline,
} from '@vicons/ionicons5'

import { useIsMobile } from '@/composables/useBreakpoint'
import { useConversationStore } from '@/stores/conversation'
import { useDiscussionStore } from '@/stores/discussion'
import { useLearnStore } from '@/stores/learn'

const store = useLearnStore()
const chat = useConversationStore()
const discussion = useDiscussionStore()
const isMobile = useIsMobile()

// 常驻旁白：slide 显示当前步骤讲解；quiz/demo 显示引导文案（quiz 不泄露答案）
const helperText: Record<string, string> = {
  quiz: '认真作答后再提交，我会在不透露答案的前提下引导你思考。',
  demo_3d: '这个 3D 演示可运行、可交互，动手体验更直观。',
  demo_function: '这个函数演示可运行、可交互，拖动或改动参数更直观。',
  demo_basic: '可以运行并编辑这段演示代码，动手体验更直观。',
}

const narration = computed<string>(() => {
  const s = store.currentSection
  if (!s) return ''
  if (s.type === 'slide') return store.currentStep?.text ?? ''
  return helperText[s.type] ?? ''
})

// 竖屏：旁白默认两行截断，点按展开/收起（换行后重置）
const clamped = ref(isMobile.value)
const expanded = ref(false)

function toggleExpand() {
  expanded.value = !expanded.value
}

watch(clamped, () => {
  expanded.value = false
})

// 竖屏：逐字显示（typewriter）
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

watch([narration, () => store.currentSection?.id, clamped], startTyping, { immediate: true })

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="teacher-bar" role="region" aria-label="智能老师旁白">
    <div class="teacher-bar__avatar" aria-hidden="true">师</div>

    <div class="teacher-bar__narration" @click="clamped && toggleExpand()">
      <span class="teacher-bar__label">讲解</span>
      <p class="teacher-bar__text" :class="{ 'is-clamp': clamped && !expanded }">
        {{ shown }}<span v-if="shown.length < narration.length" class="teacher-bar__caret" />
      </p>
      <n-button
        v-if="clamped"
        quaternary
        circle
        size="tiny"
        :aria-label="expanded ? '收起旁白' : '展开旁白'"
        @click.stop="toggleExpand"
      >
        <template #icon>
          <n-icon :size="14">
            <ChevronUpOutline v-if="expanded" />
            <ChevronDownOutline v-else />
          </n-icon>
        </template>
      </n-button>
    </div>

    <div class="teacher-bar__actions">
      <n-tooltip v-if="store.isSlide && narration && !discussion.active" placement="top">
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

      <!-- 讨论中：旁白切换为"讨论中"提示 + 终止讨论（回到讲解） -->
      <template v-if="discussion.active">
        <span class="teacher-bar__discussing" role="status">
          {{ discussion.streaming ? '老师正在讲解…' : '提问打断讲解…' }}
        </span>
        <n-button type="warning" size="small" @click="discussion.stop()">
          终止讨论
        </n-button>
      </template>
      <n-button v-else type="primary" @click="chat.openPanel()">
        <template #icon>
          <n-icon><ChatbubbleEllipsesOutline /></n-icon>
        </template>
        {{ isMobile ? '提问' : '向老师提问' }}
      </n-button>
    </div>
  </div>
</template>

<style scoped>
.teacher-bar {
  flex: 0 0 120px;
  min-height: 0;
  display: flex;
  align-items: flex-start;
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
  min-height: 0;
  display: flex;
  align-items: flex-start;
  gap: 8px;
  overflow-y: auto;
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
.teacher-bar__text.is-clamp {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  overflow: hidden;
  -webkit-line-clamp: 2;
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

.teacher-bar__discussing {
  flex: none;
  font-size: 12px;
  color: var(--app-primary, #14b8a6);
  white-space: nowrap;
}
.is-muted {
  color: var(--app-text-2, #64748b);
}

/* ---------- 移动端竖屏：压缩旁白为一行高度的收合面板 ---------- */
@media (max-width: 768px) {
  .teacher-bar {
    flex: 0 0 auto;
    gap: 8px;
    padding: 6px 10px;
  }
  .teacher-bar__avatar {
    width: 28px;
    height: 28px;
    font-size: 12px;
  }
  .teacher-bar__label {
    display: none;
  }
  .teacher-bar__text {
    font-size: 13px;
    line-height: 1.5;
  }
  .teacher-bar__narration {
    cursor: default;
  }
}
</style>
