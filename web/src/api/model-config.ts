import { request, requestVoid } from './http'

/** 后端 ModelConfigInfo：{ id, kind, provider, model, base_url, api_key_masked, is_default } */
export interface ModelConfigInfo {
  id: string
  kind: string
  provider: string
  model: string
  base_url: string
  api_key_masked: string
  is_default: boolean
}

/** 模型用途：llm=大纲/内容/讲解；image=文生图 */
export type ModelKind = 'llm' | 'image'

export const MODEL_KINDS: { value: ModelKind; label: string; desc: string }[] = [
  { value: 'llm', label: 'LLM 对话', desc: '用于大纲、内容与讲解生成' },
  { value: 'image', label: '文生图', desc: '用于 slide 幻灯片配图生成' },
]

export const KIND_LABEL: Record<string, string> = {
  llm: 'LLM',
  image: '文生图',
}

/** 新增模型配置请求体 */
export interface CreateModelConfigPayload {
  kind: ModelKind
  provider: string
  model: string
  base_url: string
  api_key: string
  is_default: boolean
}

/** 编辑模型配置请求体；字段缺省表示不改（api_key 留空表示不覆盖） */
export interface UpdateModelConfigPayload {
  kind?: ModelKind
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
  /** 该供应商适用的用途 */
  kinds: ModelKind[]
}

export const PROVIDER_PRESETS: ProviderPreset[] = [
  {
    value: 'openai',
    label: 'OpenAI',
    baseUrl: 'https://api.openai.com/v1',
    locked: false,
    kinds: ['llm', 'image'],
  },
  {
    value: 'deepseek',
    label: 'DeepSeek',
    baseUrl: 'https://api.deepseek.com/v1',
    locked: true,
    kinds: ['llm'],
  },
  {
    value: 'bailian',
    label: '阿里云百炼',
    baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    locked: false,
    kinds: ['llm', 'image'],
  },
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
