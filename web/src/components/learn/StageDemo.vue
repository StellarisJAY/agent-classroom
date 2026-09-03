<script setup lang="ts">
import { computed } from 'vue'
import { NAlert, NButton, NIcon } from 'naive-ui'
import { CodeSlashOutline, PlayOutline, PencilOutline } from '@vicons/ionicons5'

import type { DemoContent } from '@/api/learn'
import { DemoSubtype } from '@/api/learn'
import { useLearnStore } from '@/stores/learn'

const store = useLearnStore()

const demo = computed<DemoContent | null>(() => store.demoSectionContent)
const isBasic = computed(() => demo.value?.subtype === DemoSubtype.Basic)
const runCode = computed(() => (store.demoEditing ? store.demoDraft : (demo.value?.code ?? '')))
</script>

<template>
  <div class="stage-demo">
    <div v-if="!demo" class="stage-demo__empty">本环节暂无演示内容。</div>

    <template v-else>
      <div class="stage-demo__head">
        <span class="stage-demo__subtype">{{ demo.subtype }}</span>
        <span class="stage-demo__hint">
          {{
            isBasic
              ? '可交互演示：改动代码后运行预览'
              : '该类型（' + demo.subtype + '）需预注入库，本期暂不支持在线编辑'
          }}
        </span>
        <div v-if="store.demoEditing" class="stage-demo__actions">
          <n-button size="small" @click="store.cancelEditing()">取消</n-button>
          <n-button size="small" type="primary" @click="store.saveDemo()">保存</n-button>
        </div>
        <div v-else-if="isBasic" class="stage-demo__actions">
          <n-button size="small" quaternary aria-label="编辑代码" @click="store.startEditing()">
            <template #icon>
              <n-icon><PencilOutline /></n-icon>
            </template>
            编辑
          </n-button>
        </div>
      </div>

      <n-alert
        v-if="!isBasic"
        type="warning"
        :bordered="false"
        class="stage-demo__alert"
        title="演示类型暂未支持"
      >
        {{ demo.subtype }} 演示需要在沙箱内预注入
        {{ demo.subtype === '3d' ? 'Three.js' : '绘图' }} 运行库，本期尚未实现。
      </n-alert>

      <div class="stage-demo__split" :class="{ 'is-editing': store.demoEditing }">
        <div v-if="isBasic" class="stage-demo__preview">
          <div class="stage-demo__preview-head">
            <n-icon><PlayOutline /></n-icon>
            <span>运行结果</span>
          </div>
          <iframe
            :key="store.demoEditing ? 'edit' : demo.code"
            class="stage-demo__frame"
            title="演示运行沙箱"
            sandbox="allow-scripts"
            :srcdoc="runCode"
          />
        </div>

        <div v-if="store.demoEditing" class="stage-demo__editor">
          <div class="stage-demo__preview-head">
            <n-icon><CodeSlashOutline /></n-icon>
            <span>HTML 代码</span>
          </div>
          <textarea
            v-model="store.demoDraft"
            class="stage-demo__code"
            spellcheck="false"
            aria-label="演示代码编辑器"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.stage-demo {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow: hidden;
}

.stage-demo__empty {
  padding: 32px;
  text-align: center;
  color: var(--app-text-2, #64748b);
}

.stage-demo__head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.stage-demo__subtype {
  padding: 0 8px;
  font-size: 12px;
  border-radius: 999px;
  color: #0d9488;
  border: 1px solid currentColor;
}
.stage-demo__hint {
  font-size: 13px;
  color: var(--app-text-2, #64748b);
}
.stage-demo__actions {
  margin-left: auto;
  display: flex;
  gap: 8px;
}

.stage-demo__alert {
  max-width: 640px;
}

.stage-demo__split {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
}
.stage-demo__split.is-editing {
  grid-template-columns: 1fr 1fr;
}

.stage-demo__preview,
.stage-demo__editor {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 8px;
  overflow: hidden;
  background: var(--app-card-bg, #ffffff);
}

.stage-demo__preview-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  font-size: 12px;
  color: var(--app-text-2, #64748b);
  border-bottom: 1px solid var(--app-divider, #e2e8f0);
}

.stage-demo__frame {
  flex: 1;
  min-height: 0;
  width: 100%;
  border: 0;
  background: #fff;
}

.stage-demo__code {
  flex: 1;
  min-height: 0;
  width: 100%;
  border: 0;
  outline: none;
  resize: none;
  padding: 10px;
  font-family: 'SF Mono', SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace;
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--app-text-1, #0f172a);
  background: transparent;
}
</style>
