import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as modelConfigApi from '@/api/model-config'
import type { ModelConfigInfo } from '@/api/model-config'
import { useModelConfigStore } from '@/stores/model-config'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/api/model-config', () => ({
  listModelConfigs: vi.fn<() => Promise<ModelConfigInfo[]>>(),
  createModelConfig:
    vi.fn<
      (payload: {
        provider: string
        model: string
        base_url: string
        api_key: string
        is_default: boolean
      }) => Promise<ModelConfigInfo>
    >(),
  updateModelConfig: vi.fn<() => Promise<ModelConfigInfo>>(),
  deleteModelConfig: vi.fn<() => Promise<void>>(),
  setDefaultModelConfig: vi.fn<(id: string) => Promise<void>>(),
  PROVIDER_PRESETS: [],
}))

const api = vi.mocked(modelConfigApi)

function cfg(partial: Partial<ModelConfigInfo>): ModelConfigInfo {
  return {
    id: '',
    provider: 'openai',
    model: 'gpt-4o-mini',
    base_url: 'https://api.openai.com/v1',
    api_key_masked: 'sk-****',
    is_default: false,
    ...partial,
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
})

describe('model-config store', () => {
  it('ensureLoaded 只拉取一次', async () => {
    api.listModelConfigs.mockResolvedValue([cfg({ id: 'a' })])
    const store = useModelConfigStore()
    await store.ensureLoaded()
    await store.ensureLoaded()
    expect(api.listModelConfigs).toHaveBeenCalledTimes(1)
    expect(store.configs).toHaveLength(1)
    expect(store.hasConfig).toBe(true)
  })

  it('defaultConfig 反映 is_default', async () => {
    api.listModelConfigs.mockResolvedValue([cfg({ id: 'a' }), cfg({ id: 'b', is_default: true })])
    const store = useModelConfigStore()
    await store.ensureLoaded()
    expect(store.defaultConfig?.id).toBe('b')
  })

  it('setDefault 本地互斥更新', async () => {
    api.listModelConfigs.mockResolvedValue([cfg({ id: 'a', is_default: true }), cfg({ id: 'b' })])
    api.setDefaultModelConfig.mockResolvedValue(undefined)
    const store = useModelConfigStore()
    await store.ensureLoaded()

    await store.setDefault('b')
    expect(store.configs.find((c) => c.id === 'a')?.is_default).toBe(false)
    expect(store.configs.find((c) => c.id === 'b')?.is_default).toBe(true)
    expect(api.setDefaultModelConfig).toHaveBeenCalledWith('b')
  })

  it('remove 删除默认项后 defaultConfig 置空', async () => {
    api.listModelConfigs.mockResolvedValue([cfg({ id: 'a', is_default: true })])
    api.deleteModelConfig.mockResolvedValue(undefined)
    const store = useModelConfigStore()
    await store.ensureLoaded()

    await store.remove('a')
    expect(store.configs).toHaveLength(0)
    expect(store.defaultConfig).toBeNull()
    expect(store.hasConfig).toBe(false)
  })

  it('create 后重新拉取列表', async () => {
    api.listModelConfigs.mockResolvedValue([])
    api.createModelConfig.mockResolvedValue(cfg({ id: 'c' }))
    const store = useModelConfigStore()
    await store.ensureLoaded()

    api.listModelConfigs.mockResolvedValue([cfg({ id: 'c' })])
    await store.create({
      provider: 'openai',
      model: 'gpt',
      base_url: 'https://api.openai.com/v1',
      api_key: 'sk-123',
      is_default: false,
    })
    expect(store.configs).toHaveLength(1)
  })
})
