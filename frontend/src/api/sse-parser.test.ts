import { describe, expect, it } from 'vitest'

import { createSseParser, type SseFrame } from '@/api/sse-parser'

function collect() {
  const frames: SseFrame[] = []
  const parser = createSseParser((frame) => frames.push(frame))
  return { frames, parser }
}

describe('createSseParser', () => {
  it('解析完整帧：id/event/data', () => {
    const { frames, parser } = collect()
    parser.feed('id: 12\nevent: job.progress\ndata: {"progress":62}\n\n')
    expect(frames).toEqual([{ id: '12', event: 'job.progress', data: '{"progress":62}' }])
  })

  it('帧可跨 chunk 拆分（含行中间断开）', () => {
    const { frames, parser } = collect()
    parser.feed('id: 12\nevent: job.pro')
    parser.feed('gress\ndata: {"pro')
    parser.feed('gress":62}\n')
    parser.feed('\n')
    expect(frames).toHaveLength(1)
    expect(frames[0]).toEqual({ id: '12', event: 'job.progress', data: '{"progress":62}' })
  })

  it('多行 data 以 \\n 连接；空行只分发一次', () => {
    const { frames, parser } = collect()
    parser.feed('event: job.progress\ndata: 第一行\ndata: 第二行\n\n\n\n')
    expect(frames).toEqual([{ event: 'job.progress', data: '第一行\n第二行', id: undefined }])
  })

  it('兼容 CRLF 行尾', () => {
    const { frames, parser } = collect()
    parser.feed('id: 7\r\nevent: heartbeat\r\ndata: {}\r\n\r\n')
    expect(frames).toEqual([{ id: '7', event: 'heartbeat', data: '{}' }])
  })

  it('冒号开头的注释行（保活）被忽略', () => {
    const { frames, parser } = collect()
    parser.feed(': keep-alive\n\n: another\nid: 3\nevent: job.started\ndata: {}\n\n')
    expect(frames).toHaveLength(1)
    expect(frames[0].id).toBe('3')
  })

  it('id 缓冲区跨帧保持：无 id 行的帧带最近 id（契约心跳无 id）', () => {
    const { frames, parser } = collect()
    parser.feed('id: 99\nevent: job.progress\ndata: {}\n\n')
    parser.feed('event: heartbeat\ndata: {"server_time":"t"}\n\n')
    expect(frames[1]).toEqual({ id: '99', event: 'heartbeat', data: '{"server_time":"t"}' })
  })

  it('没有 data 的帧被丢弃', () => {
    const { frames, parser } = collect()
    parser.feed('event: job.progress\n\nid: 5\nevent: job.started\ndata: {}\n\n')
    expect(frames).toHaveLength(1)
    expect(frames[0].event).toBe('job.started')
  })

  it('缺省 event 为 message；含 \\0 的 id 被忽略', () => {
    const { frames, parser } = collect()
    parser.feed('data: hello\n\n')
    parser.feed('id: a\0b\ndata: world\n\n')
    expect(frames[0]).toEqual({ id: undefined, event: 'message', data: 'hello' })
    expect(frames[1]).toEqual({ id: undefined, event: 'message', data: 'world' })
  })

  it('end() 冲刷无尾换行的末行与未分发帧', () => {
    const { frames, parser } = collect()
    parser.feed('id: 42\nevent: stream.resync_required\ndata: {"reason":"history_expired"}')
    expect(frames).toHaveLength(0)
    parser.end()
    expect(frames).toHaveLength(1)
    expect(frames[0].id).toBe('42')
  })

  it('一条 chunk 含多帧', () => {
    const { frames, parser } = collect()
    parser.feed(
      'id: 1\nevent: job.queued\ndata: {}\n\nid: 2\nevent: job.started\ndata: {}\n\nevent: heartbeat\ndata: {}\n\n',
    )
    expect(frames.map((frame) => frame.event)).toEqual(['job.queued', 'job.started', 'heartbeat'])
    expect(frames.map((frame) => frame.id)).toEqual(['1', '2', '2'])
  })
})
