import { createRouter, createWebHistory } from 'vue-router'

import { getToken } from '@/api/token'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { public: true, title: '登录' },
    },
    {
      path: '/',
      component: () => import('@/layouts/MainLayout.vue'),
      meta: { title: '智能体课堂' },
      children: [
        {
          path: '',
          name: 'course-list',
          component: () => import('@/views/CourseListView.vue'),
          meta: { title: '课程库' },
        },
      ],
    },
    {
      path: '/course/:courseId/learn',
      component: () => import('@/layouts/LearnLayout.vue'),
      children: [
        {
          path: '',
          name: 'learn',
          component: () => import('@/views/LearnView.vue'),
          meta: { title: '学习' },
        },
      ],
    },
    {
      path: '/create',
      name: 'create',
      component: () => import('@/layouts/GenerateLayout.vue'),
      children: [
        {
          path: '',
          name: 'create-input',
          component: () => import('@/views/CreateView.vue'),
          meta: { title: '创建课程' },
        },
      ],
    },
    {
      path: '/preview/:courseId',
      component: () => import('@/layouts/GenerateLayout.vue'),
      children: [
        {
          path: '',
          name: 'generate',
          component: () => import('@/views/GenerateView.vue'),
          meta: { title: '生成课程' },
        },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

/** 全局前置守卫：受保护路由需登录态；已登录访问 /login 则回首页 */
router.beforeEach((to) => {
  const isPublic = to.meta.public === true
  if (isPublic) {
    if (to.path === '/login' && getToken()) return '/'
    return true
  }
  if (!getToken()) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
