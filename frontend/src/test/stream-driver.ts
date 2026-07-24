import type { QueryClient } from '@tanstack/react-query'

import type { Job, JobStreamData } from '@/api/generated/model'
import { JobStreamClient, type JobStreamTransport } from '@/api/job-stream'
import type { SseFrame } from '@/api/sse-parser'
import { isJobActive } from '@/lib/status'
import { __testPeekJobs, jobStreamMockController } from '@/mocks/handlers'

/**
 * 组件/集成测试用任务流驱动：内存传输 + 真 JobStreamClient，
 * 走完整的「SSE 帧 → 游标/合并/精准失效 → REST 重取」路径，
 * 不绕过客户端语义直接改缓存。
 *
 * emitTerminal 同时做两件事，模拟生产里 Worker 的两个副作用：
 * 1. jobStreamMockController.push —— MSW 侧状态桥把事件写进任务存储；
 * 2. transport.emit —— 客户端通过（fake）传输收到广播帧。
 */

export class FakeStreamTransport implements JobStreamTransport {
  /** 每次连接的游标（长度即连接次数）。 */
  readonly cursors: Array<string | null> = []
  private callbacks?: { onOpen: () => void; onFrame: (frame: SseFrame) => void }
  private settle?: { resolve: () => void; reject: (error: unknown) => void }

  open(
    cursor: string | null,
    _signal: AbortSignal,
    callbacks: { onOpen: () => void; onFrame: (frame: SseFrame) => void },
  ): Promise<void> {
    this.cursors.push(cursor)
    this.callbacks = callbacks
    callbacks.onOpen()
    return new Promise<void>((resolve, reject) => {
      this.settle = { resolve, reject }
    })
  }

  emit(frame: SseFrame): void {
    this.callbacks?.onFrame(frame)
  }

  /** 服务端正常关闭流（模拟断线）。 */
  end(): void {
    this.settle?.resolve()
  }

  /** 网络层错误（模拟连接失败）。 */
  fail(error: unknown): void {
    this.settle?.reject(error)
  }
}

export interface TestJobStream {
  transport: FakeStreamTransport
  stop: () => void
  /** 推送指定任务的终态事件（默认成功）。 */
  emitTerminal(
    jobId: string,
    options?: { event?: 'job.succeeded' | 'job.failed' | 'job.cancelled'; message?: string },
  ): void
  /** 按 kind + aggregate_id 定位 MSW 任务存储中的活动任务并推送终态事件。 */
  settleActiveJob(
    kind: Job['kind'],
    aggregateId: string,
    event?: 'job.succeeded' | 'job.failed' | 'job.cancelled',
  ): void
}

export function startTestJobStream(queryClient: QueryClient): TestJobStream {
  const transport = new FakeStreamTransport()
  const client = new JobStreamClient({ queryClient, transport, storage: null })
  client.start()
  let eventSeq = 900

  const emitTerminal: TestJobStream['emitTerminal'] = (jobId, options = {}) => {
    eventSeq += 1
    const event = options.event ?? 'job.succeeded'
    const status =
      event === 'job.succeeded' ? 'succeeded' : event === 'job.failed' ? 'failed' : 'cancelled'
    const data: JobStreamData = {
      job_id: jobId,
      status,
      stage: status === 'succeeded' ? '完成' : '失败',
      progress: status === 'succeeded' ? 100 : 60,
      message: options.message ?? null,
      occurred_at: new Date().toISOString(),
    }
    // Worker 写库（MSW 状态桥）→ 再广播事件（客户端 fake 传输）
    jobStreamMockController.push({ id: eventSeq, event, data })
    transport.emit({ id: String(eventSeq), event, data: JSON.stringify(data) })
  }

  return {
    transport,
    stop: () => client.stop(),
    emitTerminal,
    settleActiveJob(kind, aggregateId, event) {
      const job = __testPeekJobs().find(
        (item) =>
          item.kind === kind && item.aggregate_id === aggregateId && isJobActive(item.status),
      )
      if (!job) {
        throw new Error(`MSW 任务存储中找不到活动任务：${kind} / ${aggregateId}`)
      }
      emitTerminal(job.id, { event })
    },
  }
}
