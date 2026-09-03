<script setup lang="ts">
// MainLayout：顶部导航栏布局（占位骨架，组件落地后填充）
</script>

<template>
  <div class="main-layout">
    <header class="main-layout__nav">智能体课堂</header>
    <main class="main-layout__body">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.main-layout {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.main-layout__nav {
  height: 56px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding: 0 24px;
  border-bottom: 1px solid var(--n-divider-color, #e2e8f0);
}
.main-layout__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
</style>
