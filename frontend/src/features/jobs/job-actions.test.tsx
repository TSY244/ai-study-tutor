import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Job } from '@/api/generated/model'
import { isIdempotencyKey } from '@/lib/idempotency'
import {
  JOB_FAILED_ID,
  JOB_RUNNING_ID,
  JOB_SUCCEEDED_ID,
  LESSON_CONFIDENCE_ID,
} from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { captureRequests, problemResponse } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

const URL_JOB_ID = 'b0000000-0000-4000-8000-0000000000a1'

/** 上游 detail 哨兵：链接 + Key，不可重试原因绝不回显。 */
const SENTINEL_DETAIL =
  'upstream 413: https://www.youtube.com/watch?v=secret0&utm_source=sk-ant-leaked'

/** 失败不可重试的 URL 导入任务（material_too_large）。 */
function stubNonRetryableUrlJob() {
  const now = '2026-07-25T10:00:00Z'
  const job: Job = {
    id: URL_JOB_ID,
    kind: 'lesson_import',
    aggregate_type: 'lesson',
    aggregate_id: LESSON_CONFIDENCE_ID,
    generation_family: 'lesson_generation',
    retry_of_job_id: null,
    status: 'failed',
    stage: 'Claude 提取链接内容',
    progress: 40,
    message: null,
    attempt_count: 3,
    max_attempts: 3,
    retryable: false,
    cancel_requested: false,
    error: { code: 'material_too_large', detail: SENTINEL_DETAIL, retryable: false },
    created_at: now,
    started_at: now,
    completed_at: now,
    updated_at: now,
  }
  server.use(
    http.get(`*/api/v1/jobs/${URL_JOB_ID}`, ({ request }) =>
      HttpResponse.json(job, {
        headers: { 'X-Request-ID': request.headers.get('X-Request-ID') ?? 'mock-request-id' },
      }),
    ),
    http.get(`*/api/v1/jobs/${URL_JOB_ID}/events`, () =>
      HttpResponse.json({ items: [], page_info: { next_cursor: null, has_more: false } }),
    ),
  )
}

/** 任务详情操作：取消（确认框）与重试（幂等 + 跳转新任务）。 */
describe('JobActions', () => {
  it('失败且可重试：重试创建新任务并跳转，复用同一意图的幂等键', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', `/api/v1/jobs/${JOB_FAILED_ID}/retry`)
    const { router } = renderWithRouter(`/jobs/${JOB_FAILED_ID}`)

    await user.click(await screen.findByRole('button', { name: '重试任务' }))

    // 202 -> 跳转到新任务详情；新任务排队中，retry_of 指回原任务
    await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/jobs\/[0-9a-f-]{36}$/))
    expect(router.state.location.pathname).not.toBe(`/jobs/${JOB_FAILED_ID}`)
    expect((await screen.findAllByText('排队中')).length).toBeGreaterThan(0)
    expect(screen.getByText(JOB_FAILED_ID)).toBeInTheDocument()

    expect(requests).toHaveLength(1)
    expect(isIdempotencyKey(requests[0]?.key ?? '')).toBe(true)
    stop()
  })

  it('进行中任务：确认后取消，状态变为已取消', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', `/api/v1/jobs/${JOB_RUNNING_ID}/cancel`)
    renderWithRouter(`/jobs/${JOB_RUNNING_ID}`)

    await user.click(await screen.findByRole('button', { name: '取消任务' }))
    // 确认框：焦点圈禁，需要显式确认
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('取消后任务停止执行')
    await user.click(within(dialog).getByRole('button', { name: '取消任务' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(isIdempotencyKey(requests[0]?.key ?? '')).toBe(true)
    expect(await screen.findByText('已取消')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '取消任务' })).not.toBeInTheDocument()
    stop()
  })

  it('成功任务不显示任何操作', async () => {
    renderWithRouter(`/jobs/${JOB_SUCCEEDED_ID}`)
    expect(await screen.findByText('已成功')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '重试任务' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '取消任务' })).not.toBeInTheDocument()
  })

  it('失败不可重试：原因按 code 映射文案，不回显原始 detail', async () => {
    stubNonRetryableUrlJob()
    renderWithRouter(`/jobs/${URL_JOB_ID}`)

    const actions = await screen.findByLabelText('任务操作')
    // 不可重试原因：material_too_large 的稳定文案（说明下一步操作）
    expect(actions).toHaveTextContent('不可重试')
    expect(actions).toHaveTextContent('超出可处理的长度')
    // 原始 detail 中的链接/Key 哨兵绝不渲染
    expect(actions.textContent).not.toContain('youtube.com')
    expect(actions.textContent).not.toContain('sk-ant')
    // 不可重试任务不提供重试按钮
    expect(screen.queryByRole('button', { name: '重试任务' })).not.toBeInTheDocument()
  })

  it('服务端判 job_not_retryable：错误留在详情页展示', async () => {
    const user = userEvent.setup()
    server.use(
      http.post(`*/api/v1/jobs/:jobId/retry`, () =>
        problemResponse({
          status: 409,
          code: 'job_not_retryable',
          detail: '该任务的失败类型不支持重试。',
        }),
      ),
    )
    renderWithRouter(`/jobs/${JOB_FAILED_ID}`)

    await user.click(await screen.findByRole('button', { name: '重试任务' }))
    // 页面上同时有“任务失败”告警框；重试错误单独一条
    const alerts = await screen.findAllByRole('alert')
    expect(alerts.some((alert) => alert.textContent?.includes('任务不可重试'))).toBe(true)
    // 仍在原任务详情页
    expect(screen.getByText('任务失败')).toBeInTheDocument()
  })
})
