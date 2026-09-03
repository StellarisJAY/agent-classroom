import { ref } from 'vue'
import { defineStore } from 'pinia'

/** 课程级问答会话。占位骨架，接口就绪后填充。 */
export const useConversationStore = defineStore('conversation', () => {
  // TODO: 待问答接口落地后实现
  const initialized = ref(false)

  return { initialized }
})
