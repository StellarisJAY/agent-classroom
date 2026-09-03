<script setup lang="ts">
import { computed, h, ref } from 'vue'
import { NButton, NDropdown, NIcon, type DropdownOption } from 'naive-ui'
import { LogOutOutline, SettingsOutline } from '@vicons/ionicons5'

const emit = defineEmits<{
  (e: 'open-settings'): void
  (e: 'logout'): void
}>()

const showMenu = ref(false)

const renderIcon = (icon: typeof SettingsOutline) => () => h(NIcon, null, { default: () => h(icon) })

function handleSelect(key: string) {
  showMenu.value = false
  if (key === 'settings') emit('open-settings')
  else if (key === 'logout') emit('logout')
}

const options = computed<DropdownOption[]>(() => [
  {
    label: '设置',
    key: 'settings',
    icon: renderIcon(SettingsOutline),
  },
  {
    label: '退出登录',
    key: 'logout',
    icon: renderIcon(LogOutOutline),
  },
])
</script>

<template>
  <header class="app-navbar">
    <div class="app-navbar__brand">
      <span class="app-navbar__logo" aria-hidden="true">AI</span>
      <span class="app-navbar__title">AGENT-C</span>
    </div>

    <div class="app-navbar__actions">
      <n-dropdown
        trigger="click"
        :show="showMenu"
        :options="options"
        placement="bottom-end"
        @update:show="(v: boolean) => (showMenu = v)"
        @select="handleSelect"
      >
        <n-button quaternary circle aria-label="设置" title="设置">
          <template #icon>
            <n-icon><SettingsOutline /></n-icon>
          </template>
        </n-button>
      </n-dropdown>
    </div>
  </header>
</template>

<style scoped>
.app-navbar {
  height: 56px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  border-bottom: 1px solid var(--app-divider, #e2e8f0);
  background-color: var(--app-header-bg, #ffffff);
}

.app-navbar__brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.app-navbar__logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 700;
  color: #fff;
  background: linear-gradient(135deg, var(--app-primary, #0d9488), #14b8a6);
}

.app-navbar__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}

.app-navbar__actions {
  display: flex;
  align-items: center;
}
</style>
