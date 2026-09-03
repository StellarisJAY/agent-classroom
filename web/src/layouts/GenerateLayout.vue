<script setup lang="ts">
// GenerateLayout：生成布局（stepper + 返回，占位骨架）
</script>

<template>
  <div class="generate-layout">
    <router-view />
  </div>
</template>

<style scoped>
.generate-layout {
  height: 100%;
  display: flex;
  flex-direction: column;
}
</style>
