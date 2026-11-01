import { fireEvent, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import type { JobStreamData } from '@/api/generated/model'
import { getJobStreamStatus, resetJobStreamStatus } from '@/api/job-stream-status'
import { __testPeekJobs } from '@/mocks/handlers'
import { JOB_RUNNING_ID, LESSON_BAYES_ID } from '@/mocks/fixtures'
import { captureRequests } from '@/test/mock-helpers'
import { startTestJobStream, type TestJobStream } from '@/test/stream-driver'
import { renderWithRouter } from '@/test/utils'

/**
 * F6 集成测试：SSE → Query Cache → UI 的跨页一致性。
 * 全部走真 JobStreamClient + 内存传输 + MSW（含流事件 → REST 状态桥），
 * 覆盖：就地合并（不重取）、终态精准失效、事件时间线、resync、断线重连对账。
 */

function progressFrame(
  progress: number,
  stage = '生成教师大纲',
): {
  id: string
  event: string
  data: string
} {
  const data: JobStreamData = {
    job_id: JOB_RUNNING_ID,
    status: 'running',
    stage,
    progress,
    message: null,
    occurred_at: new Date().toISOString(),
  }
  return { id: String(900 + progress), event: 'job.progress', data: JSON.stringify(data) }
}

let activeStreams: TestJobStream[] = []

function startStream(client: Parameters<typeof startTestJobStream>[0]): TestJobStream {
  const stream = startTestJobStream(client)
  activeStreams.push(stream)
  return stream
}

afterEach(() => {
  for (const stream of activeStreams) stream.stop()
  activeStreams = []
  resetJobStreamStatus()
})

describe('任务中心一致性', () => {
  it('进度事件就地更新面板（无 REST 重取），终态事件精准失效后角标归零', async () => {
    const jobsCap = captureRequests('GET', '/api/v1/jobs')
    const { client } = renderWithRouter('/dashboard')
    const trigger = await screen.findByRole('button', { name: /任务中心，1 个任务进行中/ })

    const stream = startStream(client)
    const restCallsBefore = jobsCap.requests.length

    // 进度事件：就地合并进各 ['jobs'] 缓存，不触发重取
    stream.transport.emit(progressFrame(80))
    fireEvent.click(trigger)
    await screen.findAllByText('生成教师大纲')
    expect(jobsCap.requests.length).toBe(restCallsBefore)

    // 终态事件：MSW 桥翻转任务存储 → 精准失效 → 角标随重取归零
    stream.settleActiveJob('lesson_import', LESSON_BAYES_ID)
    await screen.findByRole('button', { name: '任务中心，暂无进行中的任务' })
    // 面板内最近任务同步显示终态
    await screen.findAllByText('已成功')
    jobsCap.stop()
  })
})

describe('任务详情', () => {
  it('事件时间线游标分页渲染，SSE 进度事件就地更新进度条', async () => {
    const { client } = renderWithRouter(`/jobs/${JOB_RUNNING_ID}`)

    const bar = await screen.findByRole('progressbar', { name: '课时导入进度' })
    expect(bar).toHaveAttribute('aria-valuenow', '45')

    // 时间线第一页（20 条），含事件类型文案与阶段
    await screen.findByText('事件时间线')
    expect((await screen.findAllByText(/进度/)).length).toBeGreaterThan(0)

    // 游标分页：26 条（排队+开始+24 条进度）→ 第二页含最早事件，按钮消失
    fireEvent.click(screen.getByRole('button', { name: '加载更多' }))
    await screen.findByText('已排队')
    expect(screen.getByText('开始执行')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument(),
    )

    // SSE 进度事件：进度条就地更新
    const stream = startStream(client)
    stream.transport.emit(progressFrame(80))
    await waitFor(() =>
      expect(screen.getByRole('progressbar', { name: '课时导入进度' })).toHaveAttribute(
        'aria-valuenow',
        '80',
      ),
    )
    // 同阶段、同百分比仍应实时显示新字符计数，无需等待终态。
    for (const characters of [123, 456]) {
      const frame = progressFrame(80, 'extracting')
      const message = `Pi 正在流式生成 · 第 1 轮 · 已接收 ${characters} 字符`
      stream.transport.emit({
        ...frame,
        id: String(1000 + characters),
        data: JSON.stringify({ ...JSON.parse(frame.data), message }),
      })
      await screen.findAllByText(message)
    }
  })
})

describe('stream.resync_required', () => {
  it('更新游标、请求活动任务并失效当前资源，不当 Job Event 合并', async () => {
    const jobsCap = captureRequests('GET', '/api/v1/jobs')
    const { client } = renderWithRouter('/jobs')
    await screen.findAllByText('课时导入')

    const stream = startStream(client)
    const restCallsBefore = jobsCap.requests.length
    stream.transport.emit({
      id: '1042',
      event: 'stream.resync_required',
      data: JSON.stringify({
        reason: 'history_expired',
        high_watermark_event_id: 1042,
        active_jobs_url: '/api/v1/jobs?active=true',
      }),
    })

    // REST 对账：任务列表被重新请求
    await waitFor(() => expect(jobsCap.requests.length).toBeGreaterThan(restCallsBefore))
    expect(getJobStreamStatus().lastEventId).toBe('1042')
    expect(getJobStreamStatus().lastResyncAt).not.toBeNull()
    expect(getJobStreamStatus().eventCounts['stream.resync_required']).toBe(1)
    // 不当 Job Event：任务存储未被流改写
    expect(__testPeekJobs().find((job) => job.id === JOB_RUNNING_ID)?.status).toBe('running')
    jobsCap.stop()
  })
})

describe('断线恢复', () => {
  it('流结束后按退避自动重连，重连成功重新对账活动任务', async () => {
    const jobsCap = captureRequests('GET', '/api/v1/jobs')
    const { client } = renderWithRouter('/dashboard')
    await screen.findByRole('button', { name: /任务中心/ })

    const stream = startStream(client)
    expect(getJobStreamStatus().state).toBe('open')
    expect(stream.transport.cursors).toHaveLength(1)

    stream.transport.end() // 断线
    await waitFor(() => expect(getJobStreamStatus().state).toBe('reconnecting'))
    // 退避（1s ±25%）后自动重连，新连接带游标
    await waitFor(() => expect(stream.transport.cursors.length).toBeGreaterThanOrEqual(2), {
      timeout: 5000,
    })
    await waitFor(() => expect(getJobStreamStatus().state).toBe('open'))
    // 重连 REST 对账：活动任务列表重新请求（初始 1 次 + 对账 ≥1 次）
    await waitFor(() => expect(jobsCap.requests.length).toBeGreaterThanOrEqual(2))
    jobsCap.stop()
  }, 10000)

  it('心跳只更新连接活性，不写任务缓存、不推进游标', async () => {
    const jobsCap = captureRequests('GET', '/api/v1/jobs')
    const { client } = renderWithRouter('/dashboard')
    await screen.findByRole('button', { name: /任务中心/ })

    const stream = startStream(client)
    const restCallsBefore = jobsCap.requests.length
    stream.transport.emit({
      event: 'heartbeat',
      data: JSON.stringify({ server_time: '2026-07-19T08:30:00Z' }),
    })

    await waitFor(() => expect(getJobStreamStatus().lastHeartbeatAt).toBe('2026-07-19T08:30:00Z'))
    expect(getJobStreamStatus().lastEventId).toBeNull()
    expect(getJobStreamStatus().eventCounts.heartbeat).toBe(1)
    expect(jobsCap.requests.length).toBe(restCallsBefore)
    jobsCap.stop()
  })
})
