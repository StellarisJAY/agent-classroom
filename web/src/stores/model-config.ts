import { ref } from 'vue'
import { defineStore } from 'pinia'

/** 用户模型配置列表 + 默认模型。占位骨架，接口就绪后填充。 */
export const useModelConfigStore = defineStore('model-config', () => {
  // TODO: 待模型配置接口落地后实现
  const initialized = ref(false)

  return { initialized }
})
