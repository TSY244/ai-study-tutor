import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Job } from '@/api/generated/model'
import {
  COURSE_STATS_ID,
  JOB_RUNNING_ID,
  LESSON_BAYES_ID,
  LESSON_CONFIDENCE_ID,
} from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { renderWithRouter } from '@/test/utils'

const URL_JOB_ID = 'b0000000-0000-4000-8000-0000000000a1'
const PROCESSING_LESSON_URL = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_BAYES_ID}/notes`

/** 上游 detail 哨兵：链接 + Key + prompt 片段，任何呈现面都不得回显。 */
const SENTINEL_DETAIL =
  'upstream 403: https://www.youtube.com/watch?v=secret0&utm_source=sk-ant-leaked（prompt：提取全文）'

function failedUrlJob(overrides: Partial<Job> = {}): Job {
  const now = '2026-07-25T10:00:00Z'
  return {
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
    attempt_count: 1,
    max_attempts: 3,
    retryable: true,
    cancel_requested: false,
    error: { code: 'material_fetch_failed', detail: SENTINEL_DETAIL, retryable: true },
    created_at: now,
    started_at: now,
    completed_at: now,
    updated_at: now,
    ...overrides,
  }
}

function echoRequestId(request: Request): HeadersInit {
  return { 'X-Request-ID': request.headers.get('X-Request-ID') ?? 'mock-request-id' }
}

/** 覆盖任务详情与事件端点：返回指定的失败 URL 任务。 */
function stubJob(job: Job) {
  server.use(
    http.get(`*/api/v1/jobs/${job.id}`, ({ request }) =>
      HttpResponse.json(job, { headers: echoRequestId(request) }),
    ),
    http.get(`*/api/v1/jobs/${job.id}/events`, () =>
      HttpResponse.json({ items: [], page_info: { next_cursor: null, has_more: false } }),
    ),
  )
}

/**
 * URL 导入失败任务的 code-first 呈现：任务详情 alert、课时页任务卡、
 * 任务中心列表行都展示稳定文案，绝不回显上游 detail（链接/Key/prompt）。
 */
describe('URL 导入失败任务呈现', () => {
  it('任务失败：详情 alert 按 code 呈现，保留错误码与重试状态，不回显 detail', async () => {
    stubJob(failedUrlJob())
    renderWithRouter(`/jobs/${URL_JOB_ID}`)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('任务失败')
    // code-first 文案（material_fetch_failed + retryable → 暂时性提取失败）
    expect(alert).toHaveTextContent('暂时性问题')
    expect(alert).toHaveTextContent('重试')
    // 错误码与重试状态仍然可见
    expect(alert).toHaveTextContent('material_fetch_failed')
    expect(alert).toHaveTextContent('可重试')
    // 上游 detail 中的链接/Key/prompt 哨兵绝不渲染
    expect(alert.textContent).not.toContain('youtube.com')
    expect(alert.textContent).not.toContain('sk-ant')
    expect(screen.queryByText(/utm_source/)).not.toBeInTheDocument()
  })

  it('ActiveJobCard：失败的 URL 任务显示 code 文案而非 detail', async () => {
    stubJob(failedUrlJob({ id: JOB_RUNNING_ID }))
    renderWithRouter(PROCESSING_LESSON_URL)

    const card = await screen.findByLabelText('后台生成任务')
    expect(card).toHaveTextContent('暂时性问题')
    expect(card.textContent).not.toContain('youtube.com')
    expect(card.textContent).not.toContain('sk-ant')
  })

  it('任务中心列表行：失败的 URL 任务显示 code 文案而非 detail', async () => {
    server.use(
      http.get('*/api/v1/jobs', ({ request }) =>
        HttpResponse.json(
          { items: [failedUrlJob()], page_info: { next_cursor: null, has_more: false } },
          { headers: echoRequestId(request) },
        ),
      ),
    )
    renderWithRouter('/jobs')

    const row = await screen.findByRole('link', { name: /课时导入/ })
    expect(row).toHaveTextContent('暂时性问题')
    expect(row.textContent).not.toContain('youtube.com')
    expect(row.textContent).not.toContain('sk-ant')
  })
})
