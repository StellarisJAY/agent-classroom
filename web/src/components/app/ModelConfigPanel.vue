<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NEmpty,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NPopconfirm,
  NSelect,
  NSwitch,
  useMessage,
} from 'naive-ui'
import type { SelectOption } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { AddOutline, ArrowBackOutline, CreateOutline, TrashOutline } from '@vicons/ionicons5'

import { PROVIDER_PRESETS } from '@/api/model-config'
import type { ModelConfigInfo } from '@/api/model-config'
import { useModelConfigStore } from '@/stores/model-config'

type Mode = 'list' | 'form'

const modelConfigStore = useModelConfigStore()
const message = useMessage()

const mode = ref<Mode>('list')
/** null=新增；非 null=编辑该 id */
const editingId = ref<string | null>(null)

const formRef = ref<FormInst | null>(null)
const saving = ref(false)
const defaultBusy = ref(false)
/** 当前所选 provider 的 API 地址是否锁定（只读） */
const baseUrlLocked = ref(false)

// ---- 列表 ----
const configs = computed(() => modelConfigStore.configs)
const loading = computed(() => modelConfigStore.loading)

async function handleSetDefault(cfg: ModelConfigInfo) {
  if (cfg.is_default || defaultBusy.value) return
  defaultBusy.value = true
  try {
    await modelConfigStore.setDefault(cfg.id)
    message.success('已设为默认模型')
  } catch (e) {
    message.error(e instanceof Error ? e.message : '设置失败')
  } finally {
    defaultBusy.value = false
  }
}

async function handleDelete(cfg: ModelConfigInfo) {
  try {
    await modelConfigStore.remove(cfg.id)
    message.success('已删除配置')
  } catch (e) {
    message.error(e instanceof Error ? e.message : '删除失败')
  }
}

// ---- 表单 ----
interface FormState {
  provider: string
  model: string
  base_url: string
  api_key: string
  is_default: boolean
}

const form = reactive<FormState>({
  provider: '',
  model: '',
  base_url: '',
  api_key: '',
  is_default: false,
})

const formTitle = computed(() => (editingId.value ? '编辑模型配置' : '新增模型配置'))

/** provider 下拉选项 */
const providerOptions: SelectOption[] = PROVIDER_PRESETS.map((p) => ({
  label: `${p.label}（${p.value}）`,
  value: p.value,
}))

/** 选中/切换 provider：命中预设且有 baseUrl 则自动填；locked 则锁定；其余清空 base_url */
function applyProviderPreset(value: string) {
  const preset = PROVIDER_PRESETS.find((p) => p.value === value)
  if (preset && preset.baseUrl) {
    form.base_url = preset.baseUrl
    baseUrlLocked.value = preset.locked
  } else {
    form.base_url = ''
    baseUrlLocked.value = false
  }
}

const rules: FormRules = {
  provider: [{ required: true, message: '请选择或输入模型供应商', trigger: ['blur', 'input'] }],
  model: [{ required: true, message: '请输入模型名称', trigger: ['blur', 'input'] }],
  base_url: [
    { required: true, message: '请输入 API 地址', trigger: ['blur', 'input'] },
    {
      validator: (_rule, value: string) => {
        const v = value ?? ''
        if (!v || /^https?:\/\//.test(v)) return true
        return new Error('需以 http(s):// 开头')
      },
      trigger: ['blur', 'input'],
    },
  ],
  api_key: [
    {
      validator: (_rule, value: string) => {
        if (editingId.value || (value ?? '').trim()) return true
        return new Error('请输入 API Key')
      },
      trigger: ['blur', 'input'],
    },
  ],
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { provider: '', model: '', base_url: '', api_key: '', is_default: false })
  baseUrlLocked.value = false
  mode.value = 'form'
}

function openEdit(cfg: ModelConfigInfo) {
  editingId.value = cfg.id
  Object.assign(form, {
    provider: cfg.provider,
    model: cfg.model,
    base_url: cfg.base_url,
    api_key: '',
    is_default: cfg.is_default,
  })
  baseUrlLocked.value = PROVIDER_PRESETS.find((p) => p.value === cfg.provider)?.locked ?? false
  mode.value = 'form'
}

function backToList() {
  mode.value = 'list'
  formRef.value?.restoreValidation()
}

async function handleSubmit() {
  const instance = formRef.value
  if (!instance) return
  const valid = await instance.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    if (editingId.value) {
      await modelConfigStore.update(editingId.value, {
        provider: form.provider.trim(),
        model: form.model.trim(),
        base_url: form.base_url.trim(),
        // 编辑时 api_key 留空则不下发（保留原 key）
        ...(form.api_key.trim() ? { api_key: form.api_key.trim() } : {}),
        is_default: form.is_default,
      })
    } else {
      await modelConfigStore.create({
        provider: form.provider.trim(),
        model: form.model.trim(),
        base_url: form.base_url.trim(),
        api_key: form.api_key.trim(),
        is_default: form.is_default,
      })
    }
    message.success(formTitle.value + '成功')
    backToList()
  } catch (e) {
    message.error(e instanceof Error ? e.message : '保存失败，请重试')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  modelConfigStore.ensureLoaded().catch((e) => {
    message.error(e instanceof Error ? e.message : '加载模型配置失败')
  })
})
</script>

<template>
  <div class="model-config-panel">
    <!-- 列表态 -->
    <template v-if="mode === 'list'">
      <div class="model-config-panel__bar">
        <span class="model-config-panel__hint">
          API Key 经服务端加密存储，默认模型用于课程生成与问答。
        </span>
        <n-button type="primary" size="small" @click="openCreate">
          <template #icon>
            <n-icon><AddOutline /></n-icon>
          </template>
          新增配置
        </n-button>
      </div>

      <div v-if="loading" class="model-config-panel__status">
        <n-spin size="small" />
      </div>

      <div v-else-if="configs.length === 0" class="model-config-panel__status">
        <n-empty description="暂无模型配置">
          <template #extra>
            <n-button size="small" @click="openCreate">新增第一条配置</n-button>
          </template>
        </n-empty>
      </div>

      <ul v-else class="model-config-panel__list">
        <li v-for="cfg in configs" :key="cfg.id" class="model-config-panel__item">
          <div class="model-config-panel__info">
            <div class="model-config-panel__line">
              <span class="model-config-panel__provider">{{ cfg.provider }}</span>
              <span class="model-config-panel__model">{{ cfg.model }}</span>
            </div>
            <div class="model-config-panel__meta">
              <span class="model-config-panel__url">{{ cfg.base_url }}</span>
              <span class="model-config-panel__key">{{ cfg.api_key_masked }}</span>
            </div>
          </div>

          <div class="model-config-panel__actions">
            <span
              class="model-config-panel__default"
              :class="{ 'model-config-panel__default--on': cfg.is_default }"
              title="每个用户仅一个默认模型"
            >
              <n-switch
                size="small"
                :value="cfg.is_default"
                :disabled="cfg.is_default || defaultBusy"
                :aria-label="cfg.is_default ? '默认模型' : '设为默认模型'"
                @update:value="() => handleSetDefault(cfg)"
              />
              <span class="model-config-panel__default-text">默认</span>
            </span>

            <n-button quaternary circle size="small" aria-label="编辑" @click="openEdit(cfg)">
              <template #icon>
                <n-icon><CreateOutline /></n-icon>
              </template>
            </n-button>

            <n-popconfirm positive-text="删除" negative-text="取消" @positive-click="handleDelete(cfg)">
              <template #trigger>
                <n-button quaternary circle size="small" aria-label="删除">
                  <template #icon>
                    <n-icon><TrashOutline /></n-icon>
                  </template>
                </n-button>
              </template>
              确定删除该模型配置吗？
            </n-popconfirm>
          </div>
        </li>
      </ul>
    </template>

    <!-- 表单态 -->
    <template v-else>
      <div class="model-config-panel__bar">
        <n-button quaternary size="small" @click="backToList">
          <template #icon>
            <n-icon><ArrowBackOutline /></n-icon>
          </template>
          返回
        </n-button>
        <span class="model-config-panel__title">{{ formTitle }}</span>
      </div>

      <n-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-placement="top"
        size="small"
        class="model-config-panel__form"
      >
        <n-form-item label="模型供应商" path="provider">
          <n-select
            v-model:value="form.provider"
            :options="providerOptions"
            placeholder="选择模型供应商"
            :disabled="saving"
            @update:value="applyProviderPreset"
          />
        </n-form-item>

        <n-form-item label="模型名称" path="model">
          <n-input
            v-model:value="form.model"
            placeholder="如 gpt-4o-mini / deepseek-chat"
            :disabled="saving"
            @keyup.enter="handleSubmit"
          />
        </n-form-item>

        <n-form-item label="API 地址" path="base_url">
          <n-input
            v-model:value="form.base_url"
            placeholder="https://api.openai.com/v1"
            :disabled="saving || baseUrlLocked"
          />
        </n-form-item>

        <n-form-item label="API Key" path="api_key">
          <n-input
            v-model:value="form.api_key"
            type="password"
            show-password-on="click"
            :placeholder="editingId ? '留空则不修改' : 'sk-…'"
            :disabled="saving"
          />
        </n-form-item>

        <n-form-item label="设为默认" path="is_default">
          <n-switch
            v-model:value="form.is_default"
            :disabled="saving || (editingId !== null && configs.find((c) => c.id === editingId)?.is_default)"
          />
        </n-form-item>

        <div class="model-config-panel__footer">
          <n-button quaternary :disabled="saving" @click="backToList">取消</n-button>
          <n-button type="primary" :loading="saving" @click="handleSubmit">保存</n-button>
        </div>
      </n-form>
    </template>
  </div>
</template>

<style scoped>
.model-config-panel {
  display: flex;
  flex-direction: column;
  min-height: 300px;
}

.model-config-panel__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.model-config-panel__hint {
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}

.model-config-panel__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
}

.model-config-panel__status {
  display: flex;
  justify-content: center;
  padding: 40px 0;
}

.model-config-panel__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.model-config-panel__item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--app-divider, #e2e8f0);
  border-radius: 8px;
  transition: border-color 0.2s;
}

.model-config-panel__item:hover {
  border-color: var(--app-primary, #14b8a6);
}

.model-config-panel__info {
  flex: 1;
  min-width: 0;
}

.model-config-panel__line {
  display: flex;
  align-items: center;
  gap: 8px;
}

.model-config-panel__provider {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-primary, #0d9488);
  background: rgba(20, 184, 166, 0.1);
  padding: 1px 6px;
  border-radius: 4px;
}

.model-config-panel__model {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text-1, #0f172a);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-config-panel__meta {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 2px;
  min-width: 0;
}

.model-config-panel__url,
.model-config-panel__key {
  font-size: 12px;
  color: var(--app-text-2, #64748b);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-config-panel__url {
  max-width: 55%;
}

.model-config-panel__actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.model-config-panel__default {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-right: 6px;
}

.model-config-panel__default-text {
  font-size: 12px;
  color: var(--app-text-2, #64748b);
}

.model-config-panel__form {
  margin-top: 4px;
}

.model-config-panel__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
</style>
