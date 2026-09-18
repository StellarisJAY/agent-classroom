import { request } from './http'

/** 模型用途：llm=大纲/内容/讲解；image=文生图 */
export type ModelKind = 'llm' | 'image'

/** 平台全局可选模型（后端配置文件 model.options，不下发密钥） */
export interface ModelOption {
  /** 唯一标识；创建课程时回传并持久化到课程 */
  key: string
  kind: ModelKind
  label: string
  /** 是否为该用途的兜底模型（未选择课程模型时使用） */
  is_default: boolean
  provider: string
  model: string
}

/** 获取平台全局可选模型清单 */
export function listModelOptions(): Promise<ModelOption[]> {
  return request<ModelOption[]>({ url: '/models', method: 'get' })
}
