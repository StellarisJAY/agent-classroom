import { ApiError } from './error'
import { clearToken, getToken } from './token'
import { Code, type ApiResponse } from './types'

/**
 * 通用 SSE 消费器：原生 fetch + ReadableStream 手工按帧（空行分隔）解析。
 * 远离 axios（15s timeout、无法流式读取），复用本地 token 与 401 处理。
 * 单条流通常长于 axios 超时上限，所有调用方必须经由此模块建立 SSE。
 */

const LOGIN_PATH = '/login'

export interface SSEFrame {
  /** `event:` 行；缺省 'message' */
  event: string
  /** `data:` 行拼接内容（后端事件均为单行 JSON） */
  data: string
}

export interface SSEOptions {
  /** 用户输入体，JSON 序列化为 POST body */
  body?: unknown
  /** 中断信号（终止讨论 / 组件卸载 / 登出） */
  signal?: AbortSignal
  /** 每解析出一个完整事件帧回调一次 */
  onEvent: (frame: SSEFrame) => void
}

/**
 * POST 建立一条 SSE 流并按序消费至服务端关闭。
 * abort 时以静默结束返回（调用方自判定），其他错误以 ApiError 抛出。
 */
export async function consumePostSSE(url: string, opts: SSEOptions): Promise<void> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (token) headers.Authorization = `Bearer ${token}`

  let res: Response
  try {
    res = await fetch(url, {
      method: 'POST',
      headers,
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
      signal: opts.signal,
    })
  } catch (e) {
    if (isAbort(e)) return
    throw new ApiError(Code.Internal, '网络异常，请检查网络连接')
  }

  // 复用统一响应未授权处理（SSE 不经 axios 拦截器，这里手工对齐）
  if (res.status === 401) {
    clearToken()
    if (window.location.pathname !== LOGIN_PATH) window.location.href = LOGIN_PATH
    throw new ApiError(Code.Unauthorized, '未登录或登录已过期', 401)
  }
  if (!res.ok) {
    throw new ApiError(Code.Internal, await errMessage(res), res.status)
  }
  if (!res.body) {
    throw new ApiError(Code.Internal, '服务器不支持流式响应')
  }

  await readEventFrames(res.body, opts.onEvent)
}

/**
 * 从可读响应流里按序解析 SSE 帧直至流关闭。
 * 单独导出：mock 流（ReadableStream 模拟响应体）复用同一解析管线。
 */
export async function readEventFrames(
  body: ReadableStream<Uint8Array>,
  onEvent: (frame: SSEFrame) => void,
): Promise<void> {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      // SSE 帧以空行结尾；JSON 内容不含裸换行，按 \n\n 独占分隔即可
      let idx = buf.indexOf('\n\n')
      while (idx !== -1) {
        emitFrame(onEvent, buf.slice(0, idx))
        buf = buf.slice(idx + 2)
        idx = buf.indexOf('\n\n')
      }
    }
    // 流关闭后残留半帧忽略（服务端总是以完整帧收尾，[DONE] 由业务层约定）
    if (buf.trim()) emitFrame(onEvent, buf)
  } catch (e) {
    if (isAbort(e)) return
    throw e instanceof ApiError ? e : new ApiError(Code.Internal, '流式连接中断')
  }
}

/** 解析一帧内的 event:/data: 行并回调。 */
function emitFrame(on: (f: SSEFrame) => void, rawBlock: string): void {
  const block = rawBlock.replace(/\r/g, '')
  if (!block.trim()) return
  const frame: SSEFrame = { event: 'message', data: '' }
  const dataLines: string[] = []
  for (const line of block.split('\n')) {
    if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart())
    else if (line.startsWith('event:')) frame.event = line.slice(6).trim()
  }
  if (!dataLines.length) return
  frame.data = dataLines.join('\n')
  on(frame)
}

async function errMessage(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as ApiResponse
    return body?.message ?? '服务器内部错误'
  } catch {
    return '服务器内部错误'
  }
}

function isAbort(e: unknown): boolean {
  return e instanceof DOMException && e.name === 'AbortError'
}
