<script setup lang="ts">
import { computed } from 'vue'
import { NAlert, NButton, NIcon } from 'naive-ui'
import { CodeSlashOutline, PlayOutline, PencilOutline } from '@vicons/ionicons5'

import { SectionType, type SectionTypeValue } from '@/api/learn'
import { useLearnStore } from '@/stores/learn'

const store = useLearnStore()

const demo = computed(() => store.demoSectionContent)
const demoType = computed<SectionTypeValue | null>(() => store.demoType)
const isBasic = computed(() => demoType.value === SectionType.DemoBasic)
const runCode = computed(() => (store.demoEditing ? store.demoDraft : (demo.value?.code ?? '')))

/** 三种 demo 类型的展示信息（demo_3d/demo_function 需预注入库，本期未实现在线运行/编辑）。 */
const demoTypeLabel: Record<string, string> = {
  [SectionType.Demo3D]: '3D 演示',
  [SectionType.DemoFunction]: '函数演示',
  [SectionType.DemoBasic]: '基础演示',
}

const typeName = computed(() =>
  demoType.value ? (demoTypeLabel[demoType.value] ?? demoType.value) : '',
)
</script>

<template>
  <div class="stage-demo">
    <div v-if="!demoType" class="stage-demo__empty">本环节暂无演示内容。</div>

    <template v-else>
      <div class="stage-demo__head">
        <span class="stage-demo__type">{{ typeName }}</span>
        <span class="stage-demo__hint">
          {{
            isBasic
              ? '可交互演示：改动代码后运行预览'
              : typeName + ' 需在沙箱内预注入运行库，本期暂不支持在线编辑'
          }}
        </span>
        <div v-if="store.demoEditing" class="stage-demo__actions">
          <n-button size="small" @click="store.cancelEditing()">取消</n-button>
          <n-button size="small" type="primary" @click="store.saveDemo()">保存</n-button>
        </div>
        <div v-else-if="isBasic && demo" class="stage-demo__actions">
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
        {{ typeName }} 需要在沙箱内预注入运行库
        {{ demoType === SectionType.Demo3D ? 'Three.js' : '绘图' }} 本期尚未实现。
      </n-alert>

      <div v-if="isBasic && demo" class="stage-demo__split" :class="{ 'is-editing': store.demoEditing }">
        <div class="stage-demo__preview">
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
            :value="store.demoDraft"
            class="stage-demo__code"
            spellcheck="false"
            aria-label="演示代码编辑器"
            @update:value="(v: string) => store.setDemoDraft(v)"
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
.stage-demo__type {
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
