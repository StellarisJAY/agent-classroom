import { getToken } from './token'
import type { GenerationSection, OutlineSection } from './course'

/**
 * 大纲生成 SSE 客户端。
 * 后端事件：start / meta{title} / section{index,section} / done / error"msg"
 * 采用 fetch + ReadableStream 以携带 Authorization 头（EventSource 无法自定义请求头）。
 */

export interface OutlineStreamHandlers {
  /** 收到 meta 事件（LLM 生成的课程标题） */
  onMeta?: (title: string) => void
  /** 收到单个大纲环节 */
  onSection?: (section: OutlineSection) => void
  /** 生成完成 */
  onDone?: () => void
  /** 出错（后端 error 事件或连接/解析失败） */
  onError?: (message: string) => void
}

/** 订阅某课程的大纲生成流；该函数在流结束或出错时 resolve/reject。 */
export async function streamOutline(
  courseId: string,
  handlers: OutlineStreamHandlers,
): Promise<void> {
  const token = getToken()
  let resp: Response
  try {
    resp = await fetch(`/api/courses/${courseId}/outline/generate`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
  } catch {
    handlers.onError?.('网络异常，无法连接大纲生成')
    return
  }
  if (!resp.ok || !resp.body) {
    handlers.onError?.('连接大纲生成失败')
    return
  }

  const reader = resp.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  const flush = () => {
    let sep: number
    while ((sep = buffer.indexOf('\n\n')) >= 0) {
      const block = buffer.slice(0, sep)
      buffer = buffer.slice(sep + 2)
      handleBlock(block, handlers)
    }
  }

  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      flush()
    }
    flush()
  } catch {
    handlers.onError?.('大纲生成流中断')
  }
}

/** 解析单个 SSE 块（若干 event:/data: 行）。 */
function handleBlock(block: string, handlers: OutlineStreamHandlers): void {
  let event = ''
  const dataLines: string[] = []
  for (const line of block.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
  }
  if (dataLines.length === 0) return
  const data = dataLines.join('\n')

  switch (event) {
    case 'meta': {
      const meta = safeParse<{ title?: string }>(data)
      if (meta?.title) handlers.onMeta?.(meta.title)
      break
    }
    case 'section': {
      const payload = safeParse<{ section: OutlineSection }>(data)
      if (payload?.section) handlers.onSection?.(payload.section)
      break
    }
    case 'done':
      handlers.onDone?.()
      break
    case 'error': {
      const parsed = safeParse<string>(data)
      handlers.onError?.(typeof parsed === 'string' ? parsed : '大纲生成失败')
      break
    }
    default:
      // start 事件等忽略
      break
  }
}

function safeParse<T>(data: string): T | null {
  try {
    return JSON.parse(data) as T
  } catch {
    return null
  }
}

/**
 * 课程内容生成进度 SSE 客户端。
 * 后端事件：start / snapshot{sections} / section{index,section} / course{status} / done / error"msg"
 */
export interface GenerationStreamHandlers {
  /** 收到全量进度快照（用于进入/恢复页面时初始化） */
  onSnapshot?: (sections: GenerationSection[]) => void
  /** 单个环节状态更新 */
  onSection?: (section: GenerationSection, index: number) => void
  /** 课程进入终态（含生成完成状态） */
  onCourse?: (status: string) => void
  /** 流正常结束 */
  onDone?: () => void
  /** 出错 */
  onError?: (message: string) => void
}

/** 订阅某课程的内容生成进度；生成在后台进行，本函数仅转发进度，流结束时 resolve。 */
export async function streamGeneration(
  courseId: string,
  handlers: GenerationStreamHandlers,
): Promise<void> {
  const token = getToken()
  let resp: Response
  try {
    resp = await fetch(`/api/courses/${courseId}/generate`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
  } catch {
    handlers.onError?.('网络异常，无法连接生成进度')
    return
  }
  if (!resp.ok || !resp.body) {
    handlers.onError?.('连接生成进度失败')
    return
  }

  const reader = resp.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  const flush = () => {
    let sep: number
    while ((sep = buffer.indexOf('\n\n')) >= 0) {
      const block = buffer.slice(0, sep)
      buffer = buffer.slice(sep + 2)
      handleGenerationBlock(block, handlers)
    }
  }

  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      flush()
    }
    flush()
  } catch {
    handlers.onError?.('生成进度流中断')
  }
}

/** 解析单个内容生成 SSE 块。 */
function handleGenerationBlock(block: string, handlers: GenerationStreamHandlers): void {
  let event = ''
  const dataLines: string[] = []
  for (const line of block.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
  }
  if (dataLines.length === 0) return
  const data = dataLines.join('\n')

  switch (event) {
    case 'snapshot': {
      const p = safeParse<{ sections?: GenerationSection[] }>(data)
      if (p?.sections) handlers.onSnapshot?.(p.sections)
      break
    }
    case 'section': {
      const p = safeParse<{ index?: number; section?: GenerationSection }>(data)
      if (p?.section) handlers.onSection?.(p.section, p.index ?? -1)
      break
    }
    case 'course': {
      const p = safeParse<{ status?: string }>(data)
      handlers.onCourse?.(p?.status ?? '')
      break
    }
    case 'done':
      handlers.onDone?.()
      break
    case 'error': {
      const parsed = safeParse<string>(data)
      handlers.onError?.(typeof parsed === 'string' ? parsed : '课程内容生成失败')
      break
    }
    default:
      break
  }
}
