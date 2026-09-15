<script setup lang="ts">
// LearnLayout：学习布局（无导航栏）。全屏高度，内容区由子路由(LearnView)编排。
</script>

<template>
  <div class="learn-layout">
    <router-view />
  </div>
</template>

<style scoped>
.learn-layout {
  height: 100vh;
  height: 100dvh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  /* 竖屏全面屏：避开底部手势条 */
  padding-bottom: env(safe-area-inset-bottom, 0px);
  padding-top: env(safe-area-inset-top, 0px);
}
</style>
