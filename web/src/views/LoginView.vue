<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NButton, NCard, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { useAuthStore } from '@/stores/auth'

type Mode = 'login' | 'register'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const message = useMessage()

const mode = ref<Mode>('login')
const loading = ref(false)
const formRef = ref<FormInst | null>(null)

const form = reactive({
  account: '',
  username: '',
  email: '',
  password: '',
})

const isLogin = computed(() => mode.value === 'login')

const loginRules: FormRules = {
  account: [{ required: true, message: '请输入用户名或邮箱', trigger: ['blur', 'input'] }],
  password: [{ required: true, message: '请输入密码', trigger: ['blur', 'input'] }],
}

const registerRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: ['blur', 'input'] },
    { min: 2, max: 32, message: '用户名长度需为 2-32 位', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: ['blur', 'input'] },
    { type: 'email', message: '邮箱格式不正确', trigger: ['blur', 'input'] },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: ['blur', 'input'] },
    { min: 6, max: 72, message: '密码长度需为 6-72 位', trigger: 'blur' },
  ],
}

const rules = computed<FormRules>(() => (isLogin.value ? loginRules : registerRules))

function switchMode(next: Mode) {
  if (mode.value === next) return
  mode.value = next
  loading.value = false
  formRef.value?.restoreValidation()
}

async function handleSubmit() {
  const formInstance = formRef.value
  if (!formInstance) return
  const valid = await formInstance.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    if (isLogin.value) {
      await authStore.login({ account: form.account.trim(), password: form.password })
    } else {
      await authStore.register({
        username: form.username.trim(),
        email: form.email.trim(),
        password: form.password,
      })
      // 注册成功后自动登录，衔接主流程
      await authStore.login({ account: form.email.trim(), password: form.password })
    }
    message.success(isLogin.value ? '登录成功' : '注册成功，欢迎加入')
    const redirect = (route.query.redirect as string) || '/'
    await router.replace(redirect)
  } catch (e) {
    message.error(e instanceof Error ? e.message : '操作失败，请重试')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
      <div class="auth-page__inner">
        <header class="auth-page__header">
          <h1 class="auth-page__title">AGENT-C</h1>
          <p class="auth-page__subtitle">
            {{ isLogin ? '登录后开始学习' : '创建账号，生成你的互动课程' }}
          </p>
        </header>

        <n-card class="auth-card" :bordered="false">
          <div class="auth-card__tabs" role="tablist" aria-label="登录或注册">
            <button
              type="button"
              class="auth-card__tab"
              :class="{ 'auth-card__tab--active': isLogin }"
              role="tab"
              :aria-selected="isLogin"
              @click="switchMode('login')"
            >
              登录
            </button>
            <button
              type="button"
              class="auth-card__tab"
              :class="{ 'auth-card__tab--active': !isLogin }"
              role="tab"
              :aria-selected="!isLogin"
              @click="switchMode('register')"
            >
              注册
            </button>
          </div>

          <n-form ref="formRef" :model="form" :rules="rules" label-placement="top" size="large">
            <n-form-item v-if="!isLogin" label="用户名" path="username">
              <n-input
                v-model:value="form.username"
                placeholder="2-32 位用户名"
                :disabled="loading"
                @keyup.enter="handleSubmit"
              />
            </n-form-item>

            <n-form-item v-if="!isLogin" label="邮箱" path="email">
              <n-input
                v-model:value="form.email"
                placeholder="you@example.com"
                :disabled="loading"
                @keyup.enter="handleSubmit"
              />
            </n-form-item>

            <n-form-item v-if="isLogin" label="用户名或邮箱" path="account">
              <n-input
                v-model:value="form.account"
                placeholder="请输入用户名或邮箱"
                :disabled="loading"
                @keyup.enter="handleSubmit"
              />
            </n-form-item>

            <n-form-item label="密码" path="password">
              <n-input
                v-model:value="form.password"
                type="password"
                show-password-on="click"
                :placeholder="isLogin ? '请输入密码' : '6-72 位密码'"
                :disabled="loading"
                @keyup.enter="handleSubmit"
              />
            </n-form-item>

            <n-button
              type="primary"
              block
              size="large"
              :loading="loading"
              class="auth-card__submit"
              @click="handleSubmit"
            >
              {{ isLogin ? '登 录' : '注 册' }}
            </n-button>
          </n-form>
        </n-card>
      </div>
    </div>
</template>

<style scoped>
.auth-page {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: radial-gradient(
    circle at 50% 0%,
    rgba(20, 184, 166, 0.1),
    transparent 55%
  );
}

.auth-page__inner {
  width: 100%;
  max-width: 400px;
}

.auth-page__header {
  text-align: center;
  margin-bottom: 24px;
}

.auth-page__title {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: 0.5px;
  background: linear-gradient(90deg, #0d9488, #14b8a6);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.auth-page__subtitle {
  margin-top: 8px;
  color: var(--app-text-2, #64748b);
  font-size: 14px;
}

.auth-card {
  border-radius: 8px;
  box-shadow: 0 1px 2px rgba(2, 6, 23, 0.06), 0 8px 24px rgba(2, 6, 23, 0.05);
}

.auth-card__tabs {
  display: flex;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--app-divider, #e2e8f0);
}

.auth-card__tab {
  flex: 1;
  padding: 12px 0;
  font-size: 15px;
  color: var(--app-text-2, #64748b);
  background: none;
  border: none;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: color 0.2s, border-color 0.2s;
}

.auth-card__tab:hover {
  color: var(--app-text-1, #0f172a);
}

.auth-card__tab--active {
  color: var(--app-primary, #0d9488);
  border-bottom-color: var(--app-primary, #0d9488);
  font-weight: 600;
}

.auth-card__submit {
  margin-top: 4px;
}
</style>
