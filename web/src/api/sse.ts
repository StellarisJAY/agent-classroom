import { getToken } from './token'
import type { OutlineSection } from './course'

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
