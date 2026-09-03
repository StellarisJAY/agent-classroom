import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import * as modelConfigApi from '@/api/model-config'
import type {
  CreateModelConfigPayload,
  ModelConfigInfo,
  UpdateModelConfigPayload,
} from '@/api/model-config'

/** 用户模型配置列表 + 默认模型。 */
export const useModelConfigStore = defineStore('model-config', () => {
  const configs = ref<ModelConfigInfo[]>([])
  const loading = ref(false)
  const initialized = ref(false)

  /** 当前默认模型配置；无默认时为 null */
  const defaultConfig = computed(
    () => configs.value.find((c) => c.is_default) ?? null,
  )
  const hasConfig = computed(() => configs.value.length > 0)

  /** 拉取配置列表并覆盖本地 */
  async function fetchList() {
    loading.value = true
    try {
      configs.value = await modelConfigApi.listModelConfigs()
      initialized.value = true
    } finally {
      loading.value = false
    }
  }

  /** 首次进入时加载（幂等），返回是否已有数据 */
  async function ensureLoaded() {
    if (!initialized.value) {
      await fetchList()
    }
  }

  /** 新增配置，成功后刷新列表 */
  async function create(payload: CreateModelConfigPayload) {
    await modelConfigApi.createModelConfig(payload)
    await fetchList()
  }

  /** 编辑配置，成功后刷新列表 */
  async function update(id: string, payload: UpdateModelConfigPayload) {
    await modelConfigApi.updateModelConfig(id, payload)
    await fetchList()
  }

  /** 删除配置，成功后移除本地项（默认项被删后 defaultConfig 自动置空） */
  async function remove(id: string) {
    await modelConfigApi.deleteModelConfig(id)
    configs.value = configs.value.filter((c) => c.id !== id)
  }

  /** 设为默认：成功后本地互斥更新 is_default，避免整表重拉 */
  async function setDefault(id: string) {
    await modelConfigApi.setDefaultModelConfig(id)
    configs.value = configs.value.map((c) => ({ ...c, is_default: c.id === id }))
  }

  return {
    configs,
    loading,
    initialized,
    defaultConfig,
    hasConfig,
    fetchList,
    ensureLoaded,
    create,
    update,
    remove,
    setDefault,
  }
})
