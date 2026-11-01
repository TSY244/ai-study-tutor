import { act, screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Job } from '@/api/generated/model'
import {
  ATTEMPT_DEMO_ID,
  COURSE_STATS_ID,
  demoJobs,
  demoQuiz,
  LESSON_BAYES_ID,
  LESSON_RANDOM_ID,
} from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { captureRequests } from '@/test/mock-helpers'
import { startTestJobStream } from '@/test/stream-driver'
import { renderWithRouter } from '@/test/utils'

const LESSON_PATH = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}`

function activeJob(overrides: Partial<Job> = {}): Job {
  return {
    ...demoJobs[0],
    aggregate_id: LESSON_RANDOM_ID,
    kind: 'quiz_generate',
    status: 'running',
    stage: '生成题目',
    progress: 35,
    ...overrides,
  }
}

function stubJobs(jobs: Job[]) {
  server.use(
    http.get('*/api/v1/jobs', ({ request }) => {
      const params = new URL(request.url).searchParams
      return HttpResponse.json({
        items: jobs.filter(
          (job) =>
            (!params.has('aggregate_type') ||
              job.aggregate_type === params.get('aggregate_type')) &&
            (!params.has('aggregate_id') || job.aggregate_id === params.get('aggregate_id')) &&
            (params.get('active') !== 'true' ||
              ['queued', 'running', 'validating', 'cancel_requested'].includes(job.status)),
        ),
        page_info: { has_more: false, next_cursor: null },
      })
    }),
    http.get('*/api/v1/jobs/:jobId', ({ params }) =>
      HttpResponse.json(jobs.find((job) => job.id === params.jobId)),
    ),
  )
}

describe('课时栏目中的活动任务', () => {
  it('空测验页显示生成进度，SSE 完成后自动显示生成的测验', async () => {
    const job = activeJob()
    stubJobs([job])
    server.use(
      http.get('*/api/v1/lessons/:lessonId/quizzes', () =>
        HttpResponse.json({
          items:
            job.status === 'succeeded'
              ? [{ ...demoQuiz, question_count: demoQuiz.questions.length }]
              : [],
          page_info: { has_more: false, next_cursor: null },
        }),
      ),
    )
    const { client } = renderWithRouter(`${LESSON_PATH}/quizzes`)
    const region = await screen.findByRole('region', { name: '测验正在生成' })
    expect(within(region).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '35')
    expect(screen.queryByText('本课时还没有测验')).not.toBeInTheDocument()

    const stream = startTestJobStream(client)
    const requests = captureRequests('GET', '/api/v1/jobs')
    try {
      stream.transport.emit({
        id: '990',
        event: 'job.progress',
        data: JSON.stringify({
          job_id: job.id,
          status: 'running',
          stage: '校对题目',
          progress: 80,
          message: 'Pi 正在流式生成 · 第 1 轮 · 已接收 123 字符',
          occurred_at: new Date().toISOString(),
        }),
      })
      await waitFor(() =>
        expect(within(region).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '80'),
      )
      expect(within(region).getByRole('status')).toHaveTextContent('已接收 123 字符')
      expect(requests.requests).toHaveLength(0)

      job.status = 'succeeded'
      await act(async () => stream.emitTerminal(job.id))
      await screen.findByText(demoQuiz.title)
      await waitFor(() => expect(screen.queryByText('测验正在生成')).not.toBeInTheDocument())
      expect(screen.getByRole('button', { name: '开始作答' })).toBeInTheDocument()
    } finally {
      requests.stop()
      stream.stop()
    }
  })

  it.each(['queued', 'validating', 'cancel_requested'] as const)(
    '已有测验时仍显示 %s 的复习测验任务，过滤其他课时、任务类型与终态',
    async (status) => {
      const job = activeJob({ kind: 'review_quiz_generate', status })
      stubJobs([
        job,
        activeJob({ id: 'other-lesson', aggregate_id: LESSON_BAYES_ID }),
        activeJob({ id: 'other-kind', kind: 'lesson_export' }),
        activeJob({ id: 'finished', status: 'succeeded' }),
      ])
      renderWithRouter(`${LESSON_PATH}/quizzes`)
      const region = await screen.findByRole('region', { name: '测验正在生成' })
      expect(within(region).getAllByRole('progressbar')).toHaveLength(1)
      expect(within(region).getByText('复习测验生成')).toBeInTheDocument()
      expect(within(region).getByRole('link', { name: '查看任务详情' })).toHaveAttribute(
        'href',
        `/jobs/${job.id}`,
      )
      expect(await screen.findByText(demoQuiz.title)).toBeInTheDocument()
    },
  )

  it.each([
    ['notes', '笔记正在生成'],
    ['raw', '原文正在生成'],
    ['outline', '大纲正在生成'],
  ])('%s 在已有内容上方展示文档生成任务', async (path, title) => {
    stubJobs([activeJob({ kind: 'lesson_regenerate' })])
    renderWithRouter(`${LESSON_PATH}/${path}`)
    const region = await screen.findByRole('region', { name: title })
    expect(within(region).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '35')
    expect(await screen.findByRole('article', { name: '文档内容' })).toBeInTheDocument()
  })

  it('作答页显示本课时的批阅任务，不混入其他作答的任务', async () => {
    const job = activeJob({
      kind: 'attempt_grade',
      aggregate_type: 'attempt',
      aggregate_id: ATTEMPT_DEMO_ID,
    })
    stubJobs([job, { ...job, id: 'other-job', aggregate_id: 'other-attempt' }])
    renderWithRouter(`${LESSON_PATH}/attempts`)
    const region = await screen.findByRole('region', { name: '作答正在处理' })
    expect(within(region).getAllByRole('progressbar')).toHaveLength(1)
    expect(within(region).getByText('作答批阅')).toBeInTheDocument()
    expect(within(region).getByRole('link', { name: '查看任务详情' })).toHaveAttribute(
      'href',
      `/jobs/${job.id}`,
    )
  })
})
