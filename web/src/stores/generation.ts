import { ref } from 'vue'
import { defineStore } from 'pinia'

/** 生成流程状态。占位骨架，接口就绪后填充。 */
export const useGenerationStore = defineStore('generation', () => {
  // TODO: 待生成流程接口落地后实现
  const initialized = ref(false)

  return { initialized }
})
