/**
 * 任务事件流 MSW mock（仅 DEV 浏览器接线使用）。
 *
 * 前端 SSE 客户端是 fetch 传输（非 EventSource），因此 Service Worker
 * 可以拦截 /api/v1/jobs/stream。这里返回一条常驻 ReadableStream：
 * 连接即发心棒帧，之后由 window.__jobStreamMock.push() 注入事件，
 * closeAll() 模拟断线（客户端应按退避重连，产生新连接）。
 *
 * E2E 通过 page.evaluate 调用该控制器，确定性覆盖：实时推送、
 * 断线重连 REST 对账、stream.resync_required、心跳。
 */
import { http, HttpResponse } from 'msw'

export interface JobStreamFrameInput {
  id?: number
  event: string
  data: unknown
}

export interface JobStreamMockController {
  /** 向所有打开的连接广播一帧；无连接时缓冲，下次连接立即补发。 */
  push(frame: JobStreamFrameInput): void
  /** 关闭全部当前连接（模拟断线，触发客户端重连）。 */
  closeAll(): void
  /** 推送前回调：handlers 用它把流事件同步进 REST 状态（模拟 Worker）。 */
  onPush(listener: (frame: JobStreamFrameInput) => void): void
  /** 已接受的连接数（断言重连用）。 */
  readonly connections: number
  /** 最近一次连接携带的 Last-Event-ID（断言游标用）。 */
  readonly lastCursor: string | null
}

export function createJobStreamMock(): {
  handler: ReturnType<typeof http.get>
  controller: JobStreamMockController
} {
  const encoder = new TextEncoder()
  const openStreams = new Set<ReadableStreamDefaultController<Uint8Array>>()
  const pending: Uint8Array[] = []
  const pushListeners = new Set<(frame: JobStreamFrameInput) => void>()
  let connections = 0
  let lastCursor: string | null = null

  function encode(frame: JobStreamFrameInput): Uint8Array {
    let text = ''
    if (frame.id !== undefined) text += `id: ${frame.id}\n`
    text += `event: ${frame.event}\n`
    text += `data: ${JSON.stringify(frame.data)}\n\n`
    return encoder.encode(text)
  }

  function broadcast(chunk: Uint8Array): void {
    for (const stream of openStreams) {
      try {
        stream.enqueue(chunk)
      } catch {
        openStreams.delete(stream)
      }
    }
  }

  const controller: JobStreamMockController = {
    push(frame) {
      for (const listener of pushListeners) listener(frame)
      const chunk = encode(frame)
      if (openStreams.size === 0) {
        pending.push(chunk)
        return
      }
      broadcast(chunk)
    },
    closeAll() {
      for (const stream of openStreams) {
        try {
          stream.close()
        } catch {
          // 已关闭：忽略
        }
      }
      openStreams.clear()
    },
    onPush(listener) {
      pushListeners.add(listener)
    },
    get connections() {
      return connections
    },
    get lastCursor() {
      return lastCursor
    },
  }

  const handler = http.get('*/api/v1/jobs/stream', ({ request }) => {
    connections += 1
    lastCursor = request.headers.get('Last-Event-ID')
    let streamController: ReadableStreamDefaultController<Uint8Array>
    const body = new ReadableStream<Uint8Array>({
      start(current) {
        streamController = current
        openStreams.add(current)
        for (const chunk of pending.splice(0)) current.enqueue(chunk)
        // 连接即心跳：证明活性，不携带 id，不推进游标。
        current.enqueue(
          encode({ event: 'heartbeat', data: { server_time: new Date().toISOString() } }),
        )
      },
      cancel() {
        openStreams.delete(streamController)
      },
    })
    return new HttpResponse(body, {
      headers: {
        'Content-Type': 'text/event-stream',
        'Cache-Control': 'no-cache',
        'X-Accel-Buffering': 'no',
      },
    })
  })

  return { handler, controller }
}
