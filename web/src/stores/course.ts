import { ref } from 'vue'
import { defineStore } from 'pinia'

/** 课程列表 + 筛选条件。占位骨架，接口就绪后填充。 */
export const useCourseStore = defineStore('course', () => {
  // TODO: 待课程接口落地后实现
  const initialized = ref(false)

  return { initialized }
})
