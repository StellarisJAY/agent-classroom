<script setup lang="ts">
import { computed } from 'vue'
import {
  NAvatar,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NRadioButton,
  NRadioGroup,
  NTabPane,
  NTabs,
} from 'naive-ui'

import { useAuthStore } from '@/stores/auth'
import { useThemeStore, type ThemeMode } from '@/stores/theme'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
}>()

const authStore = useAuthStore()
const themeStore = useThemeStore()

const username = computed(() => authStore.user?.username ?? '')
const email = computed(() => authStore.user?.email ?? '')

function handleThemeChange(mode: ThemeMode) {
  themeStore.setMode(mode)
}
</script>

<template>
  <n-modal
    :show="props.show"
    preset="card"
    title="设置"
    style="width: min(520px, calc(100vw - 32px))"
    :bordered="false"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <n-tabs type="line" animated>
      <n-tab-pane name="profile" tab="用户信息">
        <div class="settings-profile">
          <div class="settings-profile__header">
            <n-avatar round size="large" class="settings-profile__avatar">
              {{ username.charAt(0).toUpperCase() || '?' }}
            </n-avatar>
          </div>
          <n-form label-placement="top" :show-feedback="false">
            <n-form-item label="用户名">
              <n-input :value="username" placeholder="用户名" />
            </n-form-item>
            <n-form-item label="邮箱">
              <n-input :value="email" placeholder="邮箱" />
            </n-form-item>
          </n-form>
          <p class="settings-profile__hint">用户名 / 邮箱修改与密码重置功能将在接口就绪后开放。</p>
        </div>
      </n-tab-pane>
      <!-- 模型配置入口暂时屏蔽：平台统一在服务端配置文件提供全局模型 -->
      <n-tab-pane name="appearance" tab="外观">
        <div class="settings-appearance">
          <n-radio-group :value="themeStore.mode" @update:value="handleThemeChange">
            <n-radio-button value="system">跟随系统</n-radio-button>
            <n-radio-button value="light">亮色</n-radio-button>
            <n-radio-button value="dark">暗色</n-radio-button>
          </n-radio-group>
        </div>
      </n-tab-pane>
    </n-tabs>
  </n-modal>
</template>

<style scoped>
.settings-profile__header {
  display: flex;
  justify-content: center;
  margin-bottom: 20px;
}

.settings-profile__avatar {
  color: #fff;
  background: linear-gradient(135deg, var(--app-primary, #0d9488), #14b8a6);
  font-weight: 600;
}

.settings-profile__hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}

.settings-appearance {
  display: flex;
  justify-content: center;
  padding: 32px 0;
}
</style>
