/**
 * 任务事件流客户端（04-api-contract.md §14）。
 *
 * 传输：fetch + ReadableStream 解析 SSE（不用 EventSource——需要携带
 * Last-Event-ID 初始游标并自控重连节奏）。同源相对路径，默认
 * /api/v1/jobs/stream；传输层可注入，测试用内存假流。
 *
 * 语义：
 * - 游标：持久化事件的 SSE id 单调递增，保存到 sessionStorage；重连时
 *   以 Last-Event-ID 头发送，服务端回放保留窗口内的后续事件。
 * - 重连：抖动指数退避（1s 起步 ×2，±25% 抖动，封顶 15s）；重连成功后
 *   请求 active=true 任务列表并失效已观察的任务查询（REST 对账）。
 * - stream.resync_required：不是 Job Event——按 high_watermark_event_id
 *   更新游标，请求活动任务列表并失效当前资源，然后随流继续。
 * - heartbeat：只更新连接活性诊断，不写 Job Cache，不改变 Last-Event-ID。
 * - 合并：PersistedJobStreamEvent 按 job_id 就地写入 Query Cache
 *   （setQueryData/setQueriesData），高频进度不触发整树重渲染；
 *   终态事件再做精准失效（任务/课时/文档/测验/作答），驱动跨页一致。
 */
import type { InfiniteData, QueryClient } from '@tanstack/react-query'

import { getJob, listJobs } from '@/api/generated/endpoints/jobs/jobs'
import type { Job, JobList, JobResyncRequired, JobStreamData } from '@/api/generated/model'
import {
  recordJobStreamError,
  recordJobStreamEvent,
  recordJobStreamHeartbeat,
  recordJobStreamResync,
  setJobStreamState,
} from '@/api/job-stream-status'
import { invalidateDocumentsChanged, invalidateJobChanged } from '@/api/invalidation'
import { queryKeys } from '@/api/query-client'
import { createSseParser, type SseFrame } from '@/api/sse-parser'
import { isJobActive } from '@/lib/status'

export const JOB_STREAM_URL = '/api/v1/jobs/stream'
export const JOB_STREAM_CURSOR_KEY = 'job-stream:last-event-id'

export interface JobStreamTransport {
  /**
   * 打开一条 SSE 连接：onOpen 在响应头就绪后调用，onFrame 逐帧回调；
   * Promise 在流正常结束时 resolve，网络/HTTP 错误时 reject。
   */
  open(
    cursor: string | null,
    signal: AbortSignal,
    callbacks: { onOpen: () => void; onFrame: (frame: SseFrame) => void },
  ): Promise<void>
}

/** 生产传输：fetch 流式读取 + 增量 SSE 解析。 */
export function createFetchStreamTransport(url: string): JobStreamTransport {
  return {
    async open(cursor, signal, { onOpen, onFrame }) {
      const headers: Record<string, string> = { Accept: 'text/event-stream' }
      if (cursor) headers['Last-Event-ID'] = cursor
      const response = await fetch(url, {
        method: 'GET',
        headers,
        cache: 'no-store',
        credentials: 'same-origin',
        signal,
      })
      if (!response.ok || !response.body) {
        throw new Error(`任务事件流响应异常：HTTP ${response.status}`)
      }
      onOpen()
      const reader = response.body.pipeThrough(new TextDecoderStream()).getReader()
      const parser = createSseParser(onFrame)
      try {
        for (;;) {
          const { done, value } = await reader.read()
          if (done) break
          if (value) parser.feed(value)
        }
        parser.end()
      } finally {
        reader.releaseLock()
      }
    },
  }
}

export interface JobStreamClientOptions {
  queryClient: QueryClient
  transport?: JobStreamTransport
  url?: string
  /** 游标持久化存储；传 null 关闭持久化（仅内存）。 */
  storage?: Pick<Storage, 'getItem' | 'setItem'> | null
  baseDelayMs?: number
  maxDelayMs?: number
  /** 注入随机源，测试可去抖动（返回 0.5 即零抖动）。 */
  random?: () => number
}

const TERMINAL_EVENTS = new Set(['job.succeeded', 'job.failed', 'job.cancelled'])

/** 把持久化流事件就地合并进 Query Cache（不重取、不整树重渲染）。 */
export function mergeJobStreamData(queryClient: QueryClient, data: JobStreamData): void {
  const patch = (job: Job): Job => ({
    ...job,
    status: data.status,
    stage: data.stage,
    progress: data.progress,
    message: data.message,
    cancel_requested: data.status === 'cancel_requested' ? true : job.cancel_requested,
    completed_at: isJobActive(data.status) ? job.completed_at : data.occurred_at,
    updated_at: data.occurred_at,
  })
  const patchList = (list: JobList): JobList => ({
    ...list,
    items: list.items.map((job) => (job.id === data.job_id ? patch(job) : job)),
  })
  queryClient.setQueryData<Job>(queryKeys.job(data.job_id), (old) => (old ? patch(old) : old))
  // ['jobs'] 前缀下有两种缓存形态：普通 JobList 与无限分页 {pages, pageParams}。
  queryClient.setQueriesData<JobList | InfiniteData<JobList>>({ queryKey: ['jobs'] }, (old) => {
    if (!old) return old
    if ('pages' in old && Array.isArray(old.pages)) {
      return { ...old, pages: old.pages.map(patchList) }
    }
    if ('items' in old && Array.isArray(old.items)) return patchList(old)
    return old
  })
}

export class JobStreamClient {
  private readonly queryClient: QueryClient
  private readonly transport: JobStreamTransport
  private readonly storage: Pick<Storage, 'getItem' | 'setItem'> | null
  private readonly baseDelayMs: number
  private readonly maxDelayMs: number
  private readonly random: () => number

  private cursor: string | null = null
  private abortController: AbortController | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private attempts = 0
  private everConnected = false
  private stopped = true
  /** 帧处理串行化：慢消费者不丢帧、不乱序。 */
  private handling: Promise<void> = Promise.resolve()

  constructor(options: JobStreamClientOptions) {
    this.queryClient = options.queryClient
    this.transport = options.transport ?? createFetchStreamTransport(options.url ?? JOB_STREAM_URL)
    this.storage = options.storage === undefined ? this.safeSessionStorage() : options.storage
    this.baseDelayMs = options.baseDelayMs ?? 1_000
    this.maxDelayMs = options.maxDelayMs ?? 15_000
    this.random = options.random ?? Math.random
  }

  start(): void {
    if (!this.stopped) return
    this.stopped = false
    this.cursor = this.readCursor()
    void this.connect()
  }

  stop(): void {
    this.stopped = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.abortController?.abort()
    this.abortController = null
    setJobStreamState('closed')
  }

  private safeSessionStorage(): Pick<Storage, 'getItem' | 'setItem'> | null {
    try {
      return typeof window !== 'undefined' ? window.sessionStorage : null
    } catch {
      return null
    }
  }

  private readCursor(): string | null {
    try {
      return this.storage?.getItem(JOB_STREAM_CURSOR_KEY) ?? null
    } catch {
      return null
    }
  }

  private setCursor(cursor: string): void {
    this.cursor = cursor
    try {
      this.storage?.setItem(JOB_STREAM_CURSOR_KEY, cursor)
    } catch {
      // 存储不可用时退化为内存游标，不影响流本身。
    }
  }

  private async connect(): Promise<void> {
    if (this.stopped) return
    const controller = new AbortController()
    this.abortController = controller
    setJobStreamState(this.everConnected ? 'reconnecting' : 'connecting', this.attempts)
    try {
      await this.transport.open(this.cursor, controller.signal, {
        onOpen: () => {
          if (this.stopped) return
          const wasReconnect = this.everConnected
          this.everConnected = true
          this.attempts = 0
          setJobStreamState('open', 0)
          if (wasReconnect) void this.reconcileAfterReconnect()
        },
        onFrame: (frame) => this.enqueueFrame(frame),
      })
      if (!this.stopped) this.scheduleReconnect()
    } catch (error) {
      if (this.stopped || controller.signal.aborted) return
      recordJobStreamError(error instanceof Error ? error.message : '任务事件流连接失败')
      this.scheduleReconnect()
    }
  }

  /** 抖动指数退避：1s、2s、4s…封顶 15s，±25% 抖动。 */
  private scheduleReconnect(): void {
    if (this.stopped) return
    const exponential = Math.min(this.baseDelayMs * 2 ** this.attempts, this.maxDelayMs)
    const jitter = exponential * 0.25 * (this.random() * 2 - 1)
    const delay = Math.max(0, Math.round(exponential + jitter))
    this.attempts += 1
    setJobStreamState('reconnecting', this.attempts)
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      void this.connect()
    }, delay)
  }

  private enqueueFrame(frame: SseFrame): void {
    this.handling = this.handling
      .then(() => this.handleFrame(frame))
      .catch((error: unknown) => {
        recordJobStreamError(error instanceof Error ? error.message : '事件处理失败')
      })
  }

  private async handleFrame(frame: SseFrame): Promise<void> {
    if (frame.event === 'heartbeat') {
      // 心跳：只更新活性，不写 Job Cache，不动游标。
      let serverTime: string | null = null
      try {
        serverTime = (JSON.parse(frame.data) as { server_time?: string }).server_time ?? null
      } catch {
        // 心跳数据异常不影响连接。
      }
      recordJobStreamHeartbeat(serverTime)
      return
    }

    if (frame.event === 'stream.resync_required') {
      // 断档恢复：不是 Job Event——更新游标并 REST 对账。
      const data = JSON.parse(frame.data) as JobResyncRequired
      const highWatermark = String(data.high_watermark_event_id)
      this.setCursor(highWatermark)
      recordJobStreamResync(highWatermark)
      await this.reconcileAfterResync()
      return
    }

    if (!frame.event.startsWith('job.')) return // 未知事件类型：向前兼容，忽略

    const data = JSON.parse(frame.data) as JobStreamData
    if (frame.id && Number.isFinite(Number(frame.id))) this.setCursor(frame.id)
    recordJobStreamEvent(frame.event, frame.id ?? null)
    mergeJobStreamData(this.queryClient, data)
    // 事件时间线只在被观察时刷新（任务详情页），避免高频进度引发重取风暴。
    void this.queryClient.invalidateQueries({
      queryKey: queryKeys.jobEvents(data.job_id),
      refetchType: 'active',
    })
    if (frame.event === 'job.queued') {
      // 新任务：就地合并无法覆盖"列表新增成员"，失效任务列表让角标/面板出现它。
      await this.queryClient.invalidateQueries({ queryKey: ['jobs'], refetchType: 'active' })
    }
    const isTerminal = TERMINAL_EVENTS.has(frame.event) || !isJobActive(data.status)
    if (isTerminal) await this.reconcileTerminalJob(data.job_id)
  }

  /** 终态对账：任务缓存精准失效 + 按聚合类型联动课时/文档/测验/作答。 */
  private async reconcileTerminalJob(jobId: string): Promise<void> {
    let job = this.queryClient.getQueryData<Job>(queryKeys.job(jobId))
    if (!job) {
      job = await this.queryClient
        .fetchQuery({ queryKey: queryKeys.job(jobId), queryFn: () => getJob(jobId), retry: 1 })
        .catch(() => undefined)
    }
    await invalidateJobChanged(this.queryClient, jobId)
    if (!job?.aggregate_id) return
    if (job.aggregate_type === 'lesson') {
      const lessonId = job.aggregate_id
      await Promise.all([
        this.queryClient.invalidateQueries({ queryKey: queryKeys.lesson(lessonId) }),
        // 课程未知时用资源前缀失效各课程的课时列表（仅活跃查询会重取）。
        this.queryClient.invalidateQueries({ queryKey: ['lessons'], refetchType: 'active' }),
        this.queryClient.invalidateQueries({ queryKey: queryKeys.quizzes(lessonId) }),
        invalidateDocumentsChanged(this.queryClient, lessonId),
      ])
    } else if (job.aggregate_type === 'attempt') {
      await Promise.all([
        this.queryClient.invalidateQueries({
          queryKey: queryKeys.attempt(job.aggregate_id),
        }),
        // 课时作答历史按 lessonId 分键，任务事件拿不到 lessonId，
        // 精准失效整个资源前缀（仅活跃查询重取），让列表徽标/分数翻新。
        this.queryClient.invalidateQueries({
          queryKey: ['lesson-attempts'],
          refetchType: 'active',
        }),
      ])
    } else if (job.aggregate_type === 'summary_evaluation') {
      await this.queryClient.invalidateQueries({
        queryKey: queryKeys.summaryEvaluation(job.aggregate_id),
      })
    }
  }

  /** 重连成功：请求活动任务列表，并刷新已观察的任务查询（断线不代表失败，以 REST 为准）。 */
  private async reconcileAfterReconnect(): Promise<void> {
    await Promise.all([
      this.queryClient.invalidateQueries({ queryKey: ['jobs'], refetchType: 'active' }),
      this.queryClient.invalidateQueries({ queryKey: ['job'], refetchType: 'active' }),
      this.queryClient
        .prefetchQuery({
          queryKey: queryKeys.jobs({ active: true }),
          queryFn: () => listJobs({ active: true }),
        })
        .catch(() => undefined),
    ])
  }

  /**
   * resync 对账：活动任务列表（契约 active_jobs_url 即 listJobs({active:true})）
   * + 当前正在观察的资源全部以 REST 为准重取。
   */
  private async reconcileAfterResync(): Promise<void> {
    await Promise.all([
      this.queryClient.invalidateQueries({ queryKey: ['jobs'], refetchType: 'active' }),
      this.queryClient.invalidateQueries({ queryKey: ['job'], refetchType: 'active' }),
      this.queryClient.invalidateQueries({ queryKey: ['lesson'], refetchType: 'active' }),
      this.queryClient.invalidateQueries({ queryKey: ['lessons'], refetchType: 'active' }),
      this.queryClient.invalidateQueries({ queryKey: ['documents'], refetchType: 'active' }),
      this.queryClient.invalidateQueries({ queryKey: ['document'], refetchType: 'active' }),
      this.queryClient.invalidateQueries({ queryKey: ['quizzes'], refetchType: 'active' }),
      this.queryClient.invalidateQueries({ queryKey: ['attempt'], refetchType: 'active' }),
      this.queryClient
        .prefetchQuery({
          queryKey: queryKeys.jobs({ active: true }),
          queryFn: () => listJobs({ active: true }),
        })
        .catch(() => undefined),
    ])
  }
}
