import { request, requestVoid } from './http'

/** 后端 ModelConfigInfo：{ id, provider, model, base_url, api_key_masked, is_default } */
export interface ModelConfigInfo {
  id: string
  provider: string
  model: string
  base_url: string
  api_key_masked: string
  is_default: boolean
}

/** 新增模型配置请求体 */
export interface CreateModelConfigPayload {
  provider: string
  model: string
  base_url: string
  api_key: string
  is_default: boolean
}

/** 编辑模型配置请求体；字段缺省表示不改（api_key 留空表示不覆盖） */
export interface UpdateModelConfigPayload {
  provider?: string
  model?: string
  base_url?: string
  api_key?: string
  is_default?: boolean
}

/** provider 预设选项。baseUrl 非空时选中自动带出；locked 为 true 时该 base_url 不可修改 */
export interface ProviderPreset {
  value: string
  label: string
  /** 自动填写的 API 地址；空串表示不自动填（需用户输入） */
  baseUrl: string
  /** 是否锁定 API 地址（只读） */
  locked: boolean
}

export const PROVIDER_PRESETS: ProviderPreset[] = [
  { value: 'openai', label: 'OpenAI', baseUrl: 'https://api.openai.com/v1', locked: false },
  { value: 'deepseek', label: 'DeepSeek', baseUrl: 'https://api.deepseek.com/v1', locked: true },
  { value: 'qwen', label: '通义千问', baseUrl: '', locked: false },
]

/** 获取当前用户全部模型配置 */
export function listModelConfigs(): Promise<ModelConfigInfo[]> {
  return request<ModelConfigInfo[]>({ url: '/model-configs', method: 'get' })
}

/** 新增模型配置 */
export function createModelConfig(payload: CreateModelConfigPayload): Promise<ModelConfigInfo> {
  return request<ModelConfigInfo>({
    url: '/model-configs',
    method: 'post',
    data: payload,
  })
}

/** 编辑模型配置 */
export function updateModelConfig(
  id: string,
  payload: UpdateModelConfigPayload,
): Promise<ModelConfigInfo> {
  return request<ModelConfigInfo>({
    url: `/model-configs/${id}`,
    method: 'put',
    data: payload,
  })
}

/** 删除模型配置 */
export function deleteModelConfig(id: string): Promise<void> {
  return requestVoid({ url: `/model-configs/${id}`, method: 'delete' })
}

/** 将指定配置设为默认 */
export function setDefaultModelConfig(id: string): Promise<void> {
  return requestVoid({ url: `/model-configs/${id}/default`, method: 'put' })
}
