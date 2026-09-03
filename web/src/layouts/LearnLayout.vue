<script setup lang="ts">
// LearnLayout：学习布局（无导航栏，占位骨架）
</script>

<template>
  <div class="learn-layout">
    <router-view />
  </div>
</template>

<style scoped>
.learn-layout {
  height: 100%;
  display: flex;
  flex-direction: column;
}
</style>
