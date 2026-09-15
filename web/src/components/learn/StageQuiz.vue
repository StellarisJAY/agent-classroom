<script setup lang="ts">
import { computed } from 'vue'

import type { Question } from '@/api/learn'
import { useLearnStore } from '@/stores/learn'

const store = useLearnStore()

const questions = computed<Question[]>(() => store.currentSection?.questions ?? [])

const verdictLabel: Record<'correct' | 'partial' | 'wrong', string> = {
  correct: '回答正确',
  partial: '回答部分正确',
  wrong: '回答错误',
}

function optionState(q: Question, i: number) {
  const isCorrect = q.answers.includes(i)
  const isSelected = store.isSelected(q.id, i)
  if (!store.quizSubmitted) return { mark: isSelected ? 'selected' : '', correct: isCorrect }
  if (isCorrect) return { mark: isSelected ? 'right-picked' : 'right', correct: true }
  if (isSelected) return { mark: 'wrong', correct: false }
  return { mark: '', correct: false }
}
</script>

<template>
  <div class="stage-quiz">
    <div class="stage-quiz__head">
      <h3 class="stage-quiz__title">{{ store.currentSection?.title }}</h3>
      <p class="stage-quiz__subtitle">一次性作答，提交后展示答案与逐项解析</p>
    </div>

    <div v-if="!questions.length" class="stage-quiz__empty">本环节暂无题目。</div>

    <div v-else class="stage-quiz__list">
      <section
        v-for="(q, qi) in questions"
        :key="q.id"
        class="stage-quiz__card"
        :aria-label="`第 ${qi + 1} 题`"
      >
        <header class="stage-quiz__qhead">
          <span class="stage-quiz__badge">{{ q.type === 'single' ? '单选' : '多选' }}</span>
          <p class="stage-quiz__stem">{{ q.stem }}</p>
          <span v-if="store.quizSubmitted" class="stage-quiz__verdict" :data-v="store.verdict(q)">
            {{ verdictLabel[store.verdict(q)] }}
          </span>
        </header>

        <div class="stage-quiz__opts" :role="q.type === 'single' ? 'radiogroup' : 'group'">
          <button
            v-for="(opt, oi) in q.options"
            :key="oi"
            type="button"
            class="stage-quiz__opt"
            :class="`is-${optionState(q, oi).mark}`"
            :role="q.type === 'single' ? 'radio' : 'checkbox'"
            :aria-checked="store.isSelected(q.id, oi)"
            :disabled="store.quizSubmitted || store.sectionLocked"
            :data-opt="store.quizSubmitted ? 'reveal' : undefined"
            :data-correct="optionState(q, oi).correct ? '1' : undefined"
            @click="store.toggleOption(q.id, oi)"
          >
            <span class="stage-quiz__opt-key">{{ String.fromCharCode(65 + oi) }}</span>
            <span class="stage-quiz__opt-text">{{ opt }}</span>
          </button>
        </div>

        <div v-if="store.quizSubmitted" class="stage-quiz__explain">
          <p v-for="(ex, ei) in q.explanations" :key="ei" class="stage-quiz__ex-line">
            <strong>{{ String.fromCharCode(65 + ei) }}.</strong>
            <span>{{ ex }}</span>
          </p>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.stage-quiz {
  height: 100%;
  min-height: 0;
  overflow-y: auto;
  padding: 4px 2px;
}

.stage-quiz__head {
  margin-bottom: 16px;
}
.stage-quiz__title {
  margin: 0 0 4px;
  font-size: 18px;
  color: var(--app-text-1, #0f172a);
}
.stage-quiz__subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--app-text-2, #64748b);
}

.stage-quiz__empty {
  padding: 32px;
  text-align: center;
  color: var(--app-text-2, #64748b);
}

.stage-quiz__list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.stage-quiz__card {
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 8px;
  padding: 14px 16px;
  background: var(--app-card-bg, #ffffff);
}

.stage-quiz__qhead {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 12px;
}
.stage-quiz__badge {
  flex: none;
  margin-top: 2px;
  padding: 0 6px;
  font-size: 11px;
  line-height: 1.6;
  border-radius: 999px;
  color: #0f766e;
  border: 1px solid currentColor;
}
.stage-quiz__stem {
  flex: 1;
  margin: 0;
  font-size: 15px;
  font-weight: 500;
  color: var(--app-text-1, #0f172a);
}
.stage-quiz__verdict {
  flex: none;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 999px;
}
.stage-quiz__verdict[data-v='correct'] {
  color: #15803d;
  background: #dcfce7;
}
.stage-quiz__verdict[data-v='partial'] {
  color: #b45309;
  background: #fef3c7;
}
.stage-quiz__verdict[data-v='wrong'] {
  color: #b91c1c;
  background: #fee2e2;
}

.stage-quiz__opts {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.stage-quiz__opt {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 8px;
  background: transparent;
  color: var(--app-text-1, #0f172a);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.15s,
    background 0.15s;
}
.stage-quiz__opt:not(:disabled):hover {
  border-color: var(--app-primary, #14b8a6);
}
.stage-quiz__opt.is-selected {
  border-color: var(--app-primary, #14b8a6);
  background: color-mix(in srgb, var(--app-primary, #14b8a6) 10%, transparent);
}

.stage-quiz__opt[data-opt='reveal'].is-right,
.stage-quiz__opt[data-opt='reveal'].is-right-picked {
  border-color: #16a34a;
  background: #f0fdf4;
  color: #14532d;
}
.stage-quiz__opt[data-opt='reveal'].is-right-picked {
  background: #dcfce7;
}
.stage-quiz__opt[data-opt='reveal'].is-wrong {
  border-color: #ef4444;
  background: #fef2f2;
  color: #7f1d1d;
}
.stage-quiz__opt[data-opt='reveal'].is-selected:not(.is-right):not(.is-wrong) {
  opacity: 0.6;
}

.stage-quiz__opt-key {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  margin-top: 1px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  background: var(--app-divider, #e2e8f0);
}
.stage-quiz__opt-text {
  flex: 1;
  font-size: 14px;
  line-height: 1.5;
}

.stage-quiz__explain {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px dashed var(--app-divider, #e2e8f0);
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.stage-quiz__ex-line {
  margin: 0;
  font-size: 13px;
  color: var(--app-text-2, #64748b);
  line-height: 1.5;
}
.stage-quiz__ex-line strong {
  color: var(--app-text-1, #0f172a);
}
</style>
