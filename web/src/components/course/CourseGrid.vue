<script setup lang="ts">
import { NButton, NEmpty, NSpin } from 'naive-ui'

import CourseCard from '@/components/course/CourseCard.vue'
import { useCourseStore } from '@/stores/course'

const store = useCourseStore()

function retry() {
  store.clearError()
  store.reload().catch(() => {})
}
</script>

<template>
  <div class="course-grid">
    <!-- 加载骨架 -->
    <div v-if="store.loading && !store.hasItems" class="course-grid__status">
      <n-spin size="small" />
    </div>

    <!-- 错误态 -->
    <div v-else-if="store.error && !store.hasItems" class="course-grid__status">
      <n-empty :description="store.error">
        <template #extra>
          <n-button size="small" type="primary" @click="retry">重新加载</n-button>
        </template>
      </n-empty>
    </div>

    <!-- 空态 -->
    <div v-else-if="!store.hasItems" class="course-grid__status">
      <n-empty description="暂无匹配的课程，去创建一个吧" />
    </div>

    <!-- 卡片网格 -->
    <div v-else class="course-grid__list">
      <course-card v-for="item in store.items" :key="item.id" :item="item" />
    </div>

    <!-- 追加加载中占位（已有一页数据） -->
    <div v-if="store.loading && store.hasItems" class="course-grid__loading-more">
      <n-spin size="small" />
    </div>
  </div>
</template>

<style scoped>
.course-grid {
  min-height: 200px;
}

.course-grid__status {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 48px 0;
  min-height: 200px;
}

.course-grid__list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.course-grid__loading-more {
  display: flex;
  justify-content: center;
  padding: 16px 0;
}
</style>
