import { QueryClient } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/api/generated/endpoints/jobs/jobs', () => ({
  getJob: vi.fn(),
  listJobs: vi.fn(),
}))

import { getJob, listJobs } from '@/api/generated/endpoints/jobs/jobs'
import type { Job, JobList, JobStreamData } from '@/api/generated/model'
import {
  JOB_STREAM_CURSOR_KEY,
  JobStreamClient,
  mergeJobStreamData,
  type JobStreamTransport,
} from '@/api/job-stream'
import { getJobStreamStatus, resetJobStreamStatus } from '@/api/job-stream-status'
import { queryKeys } from '@/api/query-client'
import type { SseFrame } from '@/api/sse-parser'

const getJobMock = vi.mocked(getJob)
const listJobsMock = vi.mocked(listJobs)

// ---------------------------------------------------------------------------
// 测试替身：内存传输 + 内存游标存储
// ---------------------------------------------------------------------------

class FakeTransport implements JobStreamTransport {
  readonly cursors: Array<string | null> = []
  /** true：open 立即拒绝（模拟建连失败，不触发 onOpen，退避持续累积）。 */
  failImmediately = false
  private callbacks?: { onOpen: () => void; onFrame: (frame: SseFrame) => void }
  private settle?: { resolve: () => void; reject: (error: unknown) => void }

  open(
    cursor: string | null,
    _signal: AbortSignal,
    callbacks: { onOpen: () => void; onFrame: (frame: SseFrame) => void },
  ): Promise<void> {
    this.cursors.push(cursor)
    if (this.failImmediately) return Promise.reject(new Error('connect refused'))
    this.callbacks = callbacks
    callbacks.onOpen()
    return new Promise<void>((resolve, reject) => {
      this.settle = { resolve, reject }
    })
  }

  emit(frame: SseFrame): void {
    this.callbacks?.onFrame(frame)
  }

  /** 服务端正常关闭流。 */
  end(): void {
    this.settle?.resolve()
  }

  /** 网络层错误。 */
  fail(error: unknown): void {
    this.settle?.reject(error)
  }
}

function memoryStorage() {
  const data = new Map<string, string>()
  return {
    data,
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => {
      data.set(key, value)
    },
  }
}

function newQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } })
}

const LESSON_JOB: Job = {
  id: 'b0000000-0000-4000-8000-000000000001',
  kind: 'lesson_import',
  aggregate_type: 'lesson',
  aggregate_id: 'a0000000-0000-4000-8000-000000000002',
  generation_family: 'lesson_generation',
  retry_of_job_id: null,
  status: 'running',
  stage: '生成学生笔记',
  progress: 45,
  message: null,
  attempt_count: 1,
  max_attempts: 3,
  retryable: false,
  cancel_requested: false,
  error: null,
  created_at: '2026-07-17T14:15:00Z',
  started_at: '2026-07-17T14:15:05Z',
  completed_at: null,
  updated_at: '2026-07-17T14:19:30Z',
}

const ATTEMPT_GRADE_JOB: Job = {
  ...LESSON_JOB,
  id: 'b0000000-0000-4000-8000-000000000009',
  kind: 'attempt_grade',
  aggregate_type: 'attempt',
  aggregate_id: 'd0000000-0000-4000-8000-000000000009',
  generation_family: null,
}

function streamData(overrides: Partial<JobStreamData> = {}): JobStreamData {
  return {
    job_id: LESSON_JOB.id,
    status: 'running',
    stage: '生成学生笔记',
    progress: 62,
    message: null,
    occurred_at: '2026-07-17T14:20:00Z',
    ...overrides,
  }
}

function emptyJobList(): JobList {
  return { items: [], page_info: { next_cursor: null, has_more: false } }
}

beforeEach(() => {
  resetJobStreamStatus()
  listJobsMock.mockResolvedValue(emptyJobList())
  getJobMock.mockResolvedValue(LESSON_JOB)
})

afterEach(() => {
  vi.useRealTimers()
})

describe('JobStreamClient 连接与游标', () => {
  it('start() 打开连接并携带持久化游标（Last-Event-ID）', () => {
    const transport = new FakeTransport()
    const storage = memoryStorage()
    storage.setItem(JOB_STREAM_CURSOR_KEY, '821')
    const client = new JobStreamClient({ queryClient: newQueryClient(), transport, storage })

    client.start()

    expect(transport.cursors).toEqual(['821'])
    expect(getJobStreamStatus().state).toBe('open')
    client.stop()
  })

  it('持久化事件推进游标并写入存储；心跳不推进游标', async () => {
    const transport = new FakeTransport()
    const storage = memoryStorage()
    const client = new JobStreamClient({ queryClient: newQueryClient(), transport, storage })
    client.start()

    transport.emit({ id: '822', event: 'job.progress', data: JSON.stringify(streamData()) })
    transport.emit({
      event: 'heartbeat',
      data: JSON.stringify({ server_time: '2026-07-18T02:30:00Z' }),
    })

    await vi.waitFor(() => {
      expect(storage.getItem(JOB_STREAM_CURSOR_KEY)).toBe('822')
    })
    const status = getJobStreamStatus()
    expect(status.lastEventId).toBe('822')
    expect(status.lastHeartbeatAt).toBe('2026-07-18T02:30:00Z')
    expect(status.eventCounts.heartbeat).toBe(1)
    expect(status.eventCounts['job.progress']).toBe(1)
    client.stop()
  })

  it('stop() 后流结束不再重连，状态为 closed', async () => {
    vi.useFakeTimers()
    const transport = new FakeTransport()
    const client = new JobStreamClient({
      queryClient: newQueryClient(),
      transport,
      storage: null,
      random: () => 0.5,
    })
    client.start()
    client.stop()
    transport.end()

    await vi.advanceTimersByTimeAsync(60_000)
    expect(transport.cursors).toHaveLength(1)
    expect(getJobStreamStatus().state).toBe('closed')
  })
})

describe('JobStreamClient 缓存合并', () => {
  it('进度事件就地合并任务详情与列表缓存，不触发重取', async () => {
    const queryClient = newQueryClient()
    const fetchSpy = vi.fn().mockResolvedValue(LESSON_JOB)
    queryClient.setQueryData(queryKeys.job(LESSON_JOB.id), LESSON_JOB)
    queryClient.setQueryData(queryKeys.jobs({ active: true }), {
      items: [LESSON_JOB],
      page_info: { next_cursor: null, has_more: false },
    } satisfies JobList)

    const transport = new FakeTransport()
    const client = new JobStreamClient({ queryClient, transport, storage: null })
    client.start()
    transport.emit({
      id: '830',
      event: 'job.progress',
      data: JSON.stringify(streamData({ progress: 62 })),
    })

    await vi.waitFor(() => {
      expect(queryClient.getQueryData<Job>(queryKeys.job(LESSON_JOB.id))?.progress).toBe(62)
    })
    const list = queryClient.getQueryData<JobList>(queryKeys.jobs({ active: true }))
    expect(list?.items[0]?.progress).toBe(62)
    expect(list?.items[0]?.status).toBe('running')
    // 就地合并不做 REST 重取
    expect(fetchSpy).not.toHaveBeenCalled()
    expect(getJobMock).not.toHaveBeenCalled()
    expect(listJobsMock).not.toHaveBeenCalled()
    // 详情缓存未被失效（无需重取）
    expect(queryClient.getQueryState(queryKeys.job(LESSON_JOB.id))?.isInvalidated).toBe(false)
    client.stop()
  })

  it('事件到达使该任务的事件时间线 Key 精准失效', async () => {
    const queryClient = newQueryClient()
    queryClient.setQueryData(queryKeys.job(LESSON_JOB.id), LESSON_JOB)
    queryClient.setQueryData(queryKeys.jobEvents(LESSON_JOB.id), { pages: [], pageParams: [] })

    const transport = new FakeTransport()
    const client = new JobStreamClient({ queryClient, transport, storage: null })
    client.start()
    transport.emit({ id: '831', event: 'job.progress', data: JSON.stringify(streamData()) })

    await vi.waitFor(() => {
      expect(queryClient.getQueryState(queryKeys.jobEvents(LESSON_JOB.id))?.isInvalidated).toBe(
        true,
      )
    })
    client.stop()
  })

  it('mergeJobStreamData：无缓存时不伪造实体；cancel_requested 与终态 completed_at', () => {
    const queryClient = newQueryClient()
    // 无缓存：不制造详情
    mergeJobStreamData(queryClient, streamData({ job_id: 'unknown-job' }))
    expect(queryClient.getQueryData(queryKeys.job('unknown-job'))).toBeUndefined()

    queryClient.setQueryData(queryKeys.job(LESSON_JOB.id), LESSON_JOB)
    mergeJobStreamData(queryClient, streamData({ status: 'cancel_requested' }))
    expect(queryClient.getQueryData<Job>(queryKeys.job(LESSON_JOB.id))?.cancel_requested).toBe(true)

    mergeJobStreamData(
      queryClient,
      streamData({ status: 'succeeded', progress: 100, occurred_at: '2026-07-17T14:30:00Z' }),
    )
    const merged = queryClient.getQueryData<Job>(queryKeys.job(LESSON_JOB.id))
    expect(merged?.status).toBe('succeeded')
    expect(merged?.completed_at).toBe('2026-07-17T14:30:00Z')
  })

  it('mergeJobStreamData：无限分页形态（{pages}）的各页就地合并', () => {
    const queryClient = newQueryClient()
    queryClient.setQueryData(queryKeys.jobs({ view: 'paged' }), {
      pages: [
        { items: [LESSON_JOB], page_info: { next_cursor: '1', has_more: true } },
        {
          items: [{ ...LESSON_JOB, id: 'other-job' }],
          page_info: { next_cursor: null, has_more: false },
        },
      ],
      pageParams: ['', '1'],
    })
    mergeJobStreamData(queryClient, streamData({ progress: 66 }))
    const paged = queryClient.getQueryData<{ pages: JobList[] }>(queryKeys.jobs({ view: 'paged' }))
    expect(paged?.pages[0]?.items[0]?.progress).toBe(66)
    expect(paged?.pages[1]?.items[0]?.progress).toBe(45) // 其他任务不受影响
  })

  it('job.queued 使任务列表失效（新成员无法就地合并）', async () => {
    const queryClient = newQueryClient()
    queryClient.setQueryData(queryKeys.jobs({ active: true }), emptyJobList())

    const transport = new FakeTransport()
    const client = new JobStreamClient({ queryClient, transport, storage: null })
    client.start()
    transport.emit({
      id: '832',
      event: 'job.queued',
      data: JSON.stringify(streamData({ status: 'queued', progress: 0 })),
    })

    await vi.waitFor(() => {
      expect(queryClient.getQueryState(queryKeys.jobs({ active: true }))?.isInvalidated).toBe(true)
    })
    client.stop()
  })
})

describe('JobStreamClient 终态对账', () => {
  it('lesson 聚合任务终态：任务/课时/课时列表/测验/文档精准失效', async () => {
    const queryClient = newQueryClient()
    queryClient.setQueryData(queryKeys.job(LESSON_JOB.id), LESSON_JOB)
    queryClient.setQueryData(queryKeys.jobs({ active: true }), emptyJobList())
    queryClient.setQueryData(queryKeys.lesson(LESSON_JOB.aggregate_id!), {})
    queryClient.setQueryData(queryKeys.lessons('course-1'), {})
    queryClient.setQueryData(queryKeys.course('course-1'), {})
    queryClient.setQueryData(queryKeys.courses({}), {})
    queryClient.setQueryData(queryKeys.quizzes(LESSON_JOB.aggregate_id!), {})
    queryClient.setQueryData(queryKeys.documents(LESSON_JOB.aggregate_id!), {})

    const transport = new FakeTransport()
    const client = new JobStreamClient({ queryClient, transport, storage: null })
    client.start()
    transport.emit({
      id: '840',
      event: 'job.succeeded',
      data: JSON.stringify(streamData({ status: 'succeeded', progress: 100 })),
    })

    await vi.waitFor(() => {
      expect(queryClient.getQueryState(queryKeys.jobs({ active: true }))?.isInvalidated).toBe(true)
    })
    expect(queryClient.getQueryState(queryKeys.job(LESSON_JOB.id))?.isInvalidated).toBe(true)
    expect(
      queryClient.getQueryState(queryKeys.lesson(LESSON_JOB.aggregate_id!))?.isInvalidated,
    ).toBe(true)
    expect(queryClient.getQueryState(queryKeys.lessons('course-1'))?.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(queryKeys.course('course-1'))?.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(queryKeys.courses({}))?.isInvalidated).toBe(true)
    expect(
      queryClient.getQueryState(queryKeys.quizzes(LESSON_JOB.aggregate_id!))?.isInvalidated,
    ).toBe(true)
    expect(
      queryClient.getQueryState(queryKeys.documents(LESSON_JOB.aggregate_id!))?.isInvalidated,
    ).toBe(true)
    // 详情已在缓存：不需要 getJob 兜底
    expect(getJobMock).not.toHaveBeenCalled()
    client.stop()
  })

  it('attempt 聚合任务终态（详情未缓存）：REST 兜底取详情后使 attempt 失效', async () => {
    const queryClient = newQueryClient()
    getJobMock.mockResolvedValue(ATTEMPT_GRADE_JOB)
    queryClient.setQueryData(queryKeys.attempt(ATTEMPT_GRADE_JOB.aggregate_id!), {})

    const transport = new FakeTransport()
    const client = new JobStreamClient({ queryClient, transport, storage: null })
    client.start()
    transport.emit({
      id: '841',
      event: 'job.succeeded',
      data: JSON.stringify(
        streamData({
          job_id: ATTEMPT_GRADE_JOB.id,
          status: 'succeeded',
          progress: 100,
          stage: '完成',
        }),
      ),
    })

    await vi.waitFor(() => {
      expect(
        queryClient.getQueryState(queryKeys.attempt(ATTEMPT_GRADE_JOB.aggregate_id!))
          ?.isInvalidated,
      ).toBe(true)
    })
    expect(getJobMock).toHaveBeenCalledWith(ATTEMPT_GRADE_JOB.id)
    client.stop()
  })
})

describe('JobStreamClient stream.resync_required', () => {
  it('按高水位更新游标、请求活动任务并失效当前资源，且不当 Job Event 合并', async () => {
    const queryClient = newQueryClient()
    queryClient.setQueryData(queryKeys.job(LESSON_JOB.id), LESSON_JOB)
    queryClient.setQueryData(queryKeys.lesson('lesson-x'), {})
    queryClient.setQueryData(queryKeys.attempt('attempt-x'), {})
    const storage = memoryStorage()

    const transport = new FakeTransport()
    const client = new JobStreamClient({ queryClient, transport, storage })
    client.start()
    transport.emit({
      id: '1042',
      event: 'stream.resync_required',
      data: JSON.stringify({
        reason: 'history_expired',
        high_watermark_event_id: 1042,
        active_jobs_url: '/api/v1/jobs?active=true',
      }),
    })

    await vi.waitFor(() => {
      expect(storage.getItem(JOB_STREAM_CURSOR_KEY)).toBe('1042')
    })
    // active_jobs_url 对账：请求活动任务列表
    expect(listJobsMock).toHaveBeenCalledWith({ active: true })
    // 当前资源以 REST 为准
    expect(queryClient.getQueryState(queryKeys.lesson('lesson-x'))?.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(queryKeys.attempt('attempt-x'))?.isInvalidated).toBe(true)
    // 不当 Job Event：缓存未被流数据改写，事件计数不计入 job.*
    expect(queryClient.getQueryData<Job>(queryKeys.job(LESSON_JOB.id))?.progress).toBe(45)
    const status = getJobStreamStatus()
    expect(status.eventCounts['stream.resync_required']).toBe(1)
    expect(status.eventCounts['job.progress']).toBeUndefined()
    expect(status.lastResyncAt).not.toBeNull()
    client.stop()
  })
})

describe('JobStreamClient 断线重连', () => {
  it('流结束后 1s 重连，带最新游标并 REST 对账；成功建连后退避重置', async () => {
    vi.useFakeTimers()
    const transport = new FakeTransport()
    const storage = memoryStorage()
    const client = new JobStreamClient({
      queryClient: newQueryClient(),
      transport,
      storage,
      random: () => 0.5, // 零抖动：退避点精确可断言
    })
    client.start()
    transport.emit({ id: '900', event: 'job.progress', data: JSON.stringify(streamData()) })
    await vi.advanceTimersByTimeAsync(0)

    transport.end() // 第一次断线
    await vi.advanceTimersByTimeAsync(999)
    expect(transport.cursors).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(transport.cursors).toEqual([null, '900']) // 1s 后重连，带游标
    expect(listJobsMock).toHaveBeenCalledWith({ active: true }) // 重连 REST 对账

    transport.end() // 再次断线：上次建连成功，退避已重置回 1s
    await vi.advanceTimersByTimeAsync(999)
    expect(transport.cursors).toHaveLength(2)
    await vi.advanceTimersByTimeAsync(1)
    expect(transport.cursors).toHaveLength(3)
    client.stop()
  })

  it('连续建连失败：指数退避 1s/2s/4s/8s，封顶 15s', async () => {
    vi.useFakeTimers()
    const transport = new FakeTransport()
    transport.failImmediately = true
    const client = new JobStreamClient({
      queryClient: newQueryClient(),
      transport,
      storage: null,
      random: () => 0.5,
    })
    client.start() // 第 1 次建连（立即失败）
    await vi.advanceTimersByTimeAsync(0)
    expect(transport.cursors).toHaveLength(1)

    // 退避序列：1s、2s、4s、8s、15s（封顶）、15s…
    for (const delay of [1_000, 2_000, 4_000, 8_000, 15_000, 15_000]) {
      const before = transport.cursors.length
      await vi.advanceTimersByTimeAsync(delay - 1)
      expect(transport.cursors).toHaveLength(before)
      await vi.advanceTimersByTimeAsync(1)
      expect(transport.cursors).toHaveLength(before + 1)
    }
    expect(getJobStreamStatus().state).toBe('reconnecting')
    client.stop()
  })

  it('传输层抛错同样进入退避重连', async () => {
    vi.useFakeTimers()
    const transport = new FakeTransport()
    const client = new JobStreamClient({
      queryClient: newQueryClient(),
      transport,
      storage: null,
      random: () => 0.5,
    })
    client.start()
    transport.fail(new Error('network down'))

    await vi.advanceTimersByTimeAsync(1_000)
    expect(transport.cursors).toHaveLength(2)
    expect(getJobStreamStatus().state).toBe('open')
    client.stop()
  })
})

describe('JobStreamClient 健壮性', () => {
  it('坏 JSON 帧不中断后续事件处理', async () => {
    const queryClient = newQueryClient()
    queryClient.setQueryData(queryKeys.job(LESSON_JOB.id), LESSON_JOB)
    const transport = new FakeTransport()
    const client = new JobStreamClient({ queryClient, transport, storage: null })
    client.start()

    transport.emit({ id: '850', event: 'job.progress', data: '{broken json' })
    transport.emit({
      id: '851',
      event: 'job.progress',
      data: JSON.stringify(streamData({ progress: 70 })),
    })

    await vi.waitFor(() => {
      expect(queryClient.getQueryData<Job>(queryKeys.job(LESSON_JOB.id))?.progress).toBe(70)
    })
    expect(getJobStreamStatus().lastError).not.toBeNull()
    client.stop()
  })

  it('慢消费者：终态对账未完成时后续帧排队，按序应用不丢帧', async () => {
    const queryClient = newQueryClient()
    // 详情未缓存 → 终态对账走 getJob 兜底；用可控 promise 阻塞它
    let releaseGetJob!: (job: Job) => void
    getJobMock.mockImplementation(
      () =>
        new Promise<Job>((resolve) => {
          releaseGetJob = resolve
        }),
    )

    const transport = new FakeTransport()
    const client = new JobStreamClient({ queryClient, transport, storage: null })
    client.start()

    transport.emit({
      id: '860',
      event: 'job.succeeded',
      data: JSON.stringify(streamData({ status: 'succeeded', progress: 100 })),
    })
    transport.emit({
      id: '861',
      event: 'job.progress',
      data: JSON.stringify(streamData({ progress: 61 })),
    })
    transport.emit({
      id: '862',
      event: 'job.progress',
      data: JSON.stringify(streamData({ progress: 62 })),
    })

    // 阻塞期间后续帧不得被应用（串行队列）
    await Promise.resolve()
    expect(getJobStreamStatus().eventCounts['job.progress']).toBeUndefined()

    releaseGetJob(LESSON_JOB)
    await vi.waitFor(() => {
      expect(getJobStreamStatus().eventCounts['job.progress']).toBe(2)
    })
    expect(getJobStreamStatus().lastEventId).toBe('862')
    client.stop()
  })
})
