<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NIcon, NPagination } from 'naive-ui'
import { AddOutline } from '@vicons/ionicons5'

import CourseFilters from '@/components/course/CourseFilters.vue'
import CourseGrid from '@/components/course/CourseGrid.vue'
import { COURSE_PAGE_SIZE, useCourseStore } from '@/stores/course'

const store = useCourseStore()
const router = useRouter()

const totalPages = computed(() => Math.max(1, Math.ceil(store.total / COURSE_PAGE_SIZE)))

onMounted(() => {
  store.reload().catch(() => {})
})

function createCourse() {
  router.push('/create')
}

function onPageChange(page: number) {
  store.goTo(page).catch(() => {})
}
</script>

<template>
  <div class="course-list">
    <div class="course-list__head">
      <div>
        <h2 class="course-list__title">课程库</h2>
        <p class="course-list__subtitle">学习或创建智能体生成的互动课程</p>
      </div>
      <n-button type="primary" @click="createCourse">
        <template #icon>
          <n-icon><AddOutline /></n-icon>
        </template>
        创建课程
      </n-button>
    </div>

    <course-filters class="course-list__filters" />

    <course-grid class="course-list__grid" />

    <div v-if="store.hasItems && totalPages > 1" class="course-list__pager">
      <n-pagination
        :page="store.page"
        :item-count="store.total"
        :page-size="COURSE_PAGE_SIZE"
        @update:page="onPageChange"
      />
    </div>
  </div>
</template>

<style scoped>
.course-list {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px;
}

.course-list__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.course-list__title {
  margin: 0 0 4px;
  font-size: 22px;
  font-weight: 700;
  color: var(--app-text-1, #0f172a);
}

.course-list__subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--app-text-2, #64748b);
}

.course-list__filters {
  margin-bottom: 20px;
}

.course-list__pager {
  display: flex;
  justify-content: center;
  padding-top: 20px;
}
</style>
