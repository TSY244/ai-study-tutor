/**
 * SSE 帧增量解析器（WHATWG Server-Sent Events 子集）。
 *
 * - 按行缓冲，空行分发一帧；支持 CRLF 与不以换行结尾的末行（end() 冲刷）。
 * - data 多行以 \n 连接；以 ':' 开头的行是注释（保活），直接丢弃。
 * - id 缓冲区跨帧保持（规范语义）；heartbeat 等无 id 行的帧会带上最近 id，
 *   是否推进游标由消费方按事件类型决定。
 * - retry 字段被忽略：重连节奏由 JobStreamClient 的抖动退避统一控制。
 */
export interface SseFrame {
  id?: string
  event: string
  data: string
}

export interface SseParser {
  feed(chunk: string): void
  end(): void
}

export function createSseParser(onFrame: (frame: SseFrame) => void): SseParser {
  let buffer = ''
  let dataLines: string[] = []
  let eventType = ''
  let lastId: string | undefined

  function dispatch(): void {
    if (dataLines.length > 0) {
      onFrame({ id: lastId, event: eventType || 'message', data: dataLines.join('\n') })
    }
    dataLines = []
    eventType = ''
  }

  function processLine(rawLine: string): void {
    const line = rawLine.endsWith('\r') ? rawLine.slice(0, -1) : rawLine
    if (line === '') {
      dispatch()
      return
    }
    if (line.startsWith(':')) return
    const colon = line.indexOf(':')
    const field = colon === -1 ? line : line.slice(0, colon)
    let value = colon === -1 ? '' : line.slice(colon + 1)
    if (value.startsWith(' ')) value = value.slice(1)
    switch (field) {
      case 'data':
        dataLines.push(value)
        break
      case 'event':
        eventType = value
        break
      case 'id':
        if (!value.includes('\u0000')) lastId = value
        break
      default:
        break
    }
  }

  return {
    feed(chunk: string): void {
      buffer += chunk
      let newlineIndex = buffer.indexOf('\n')
      while (newlineIndex !== -1) {
        processLine(buffer.slice(0, newlineIndex))
        buffer = buffer.slice(newlineIndex + 1)
        newlineIndex = buffer.indexOf('\n')
      }
    },
    end(): void {
      if (buffer !== '') {
        processLine(buffer)
        buffer = ''
      }
      dispatch()
    },
  }
}
