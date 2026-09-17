<script setup lang="ts">
/**
 * demo_3d 渲染壳：接收原始落库 JSON，经 schema 修剪后由 Demo3DScene 构建可交互场景。
 * 右侧悬浮滑块面板按控制器 title 展示；解析/结构整体失败渲染降级提示。
 * 内容变化 → 整场景 dispose + 重建（当前版本无在线编辑，重建频率极低）。
 */
import { NAlert, NSlider } from 'naive-ui'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

import type { Demo3DContent } from '@/api/learn'

import { parseDemo3DContent } from './schema'
import { Demo3DParseError } from './types'
import { buildDemo3D, type Demo3DInstance, type SliderDef } from './Demo3DScene'

const props = defineProps<{ content: Demo3DContent | null }>()

const container = ref<HTMLElement | null>(null)
const sliders = ref<SliderDef[]>([])
const failed = ref(false)

let instance: Demo3DInstance | null = null

function rebuild(): void {
  instance?.dispose()
  instance = null
  sliders.value = []
  failed.value = false
  if (!container.value) return
  if (!props.content) {
    failed.value = true
    return
  }
  try {
    const def = parseDemo3DContent(props.content)
    instance = buildDemo3D(container.value, def)
    sliders.value = instance.sliderDefs.map((d) => ({ ...d }))
  }
  catch (e) {
    if (e instanceof Demo3DParseError) {
      console.warn('demo 3d content invalid', e)
    }
    else {
      console.error('demo 3d build failed', e)
    }
    failed.value = true
  }
}

onMounted(rebuild)
watch(() => props.content, rebuild, { deep: false })
onBeforeUnmount(() => {
  instance?.dispose()
  instance = null
})

function onSlider(def: SliderDef, value: number): void {
  const c = def.control
  if (c.type === 'orbit') return
  instance?.applySlider(c.type, c.targetId, c.axis, value)
}
</script>

<template>
  <div class="demo3d">
    <n-alert
      v-if="failed"
      type="warning"
      :bordered="false"
      class="demo3d__alert"
      title="3D 场景解析失败"
    >
      演示内容不是有效的场景描述数据，无法渲染。
    </n-alert>
    <div ref="container" class="demo3d__canvas" />
    <div v-if="sliders.length > 0" class="demo3d__panel">
      <div v-for="(def, i) in sliders" :key="i" class="demo3d__control">
        <span class="demo3d__control-title">{{ def.control.title }}</span>
        <n-slider
          :min="def.min"
          :max="def.max"
          :step="def.step"
          :default-value="def.initial"
          @update:value="(v: number) => onSlider(def, v)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.demo3d {
  position: relative;
  flex: 1;
  min-height: 0;
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 8px;
  overflow: hidden;
  background: var(--app-card-bg, #ffffff);
}

.demo3d__canvas {
  position: absolute;
  inset: 0;
}

.demo3d__alert {
  position: absolute;
  top: 10px;
  left: 10px;
  right: 10px;
  z-index: 2;
}

.demo3d__panel {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 1;
  width: 220px;
  max-height: calc(100% - 24px);
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.72);
  backdrop-filter: blur(4px);
}

.demo3d__control {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.demo3d__control-title {
  font-size: 12px;
  color: #e2e8f0;
}
</style>
