import { fireEvent, screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { AttemptSummary } from '@/api/generated/model'
import {
  ATTEMPT_GRADED_ID,
  COURSE_STATS_ID,
  LESSON_BAYES_ID,
  LESSON_RANDOM_ID,
  QUIZ_DEMO_ID,
} from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { renderWithRouter } from '@/test/utils'

function echoRequestId(request: Request): HeadersInit {
  return { 'X-Request-ID': request.headers.get('X-Request-ID') ?? 'mock-request-id' }
}

function summaryRow(overrides: Partial<AttemptSummary>): AttemptSummary {
  return {
    id: 'd0000000-0000-4000-8000-000000000101',
    quiz_id: QUIZ_DEMO_ID,
    quiz_title: '随机变量与分布 · 随堂测验',
    status: 'graded',
    auto_score: 8,
    final_score: 8,
    max_score: 10,
    started_at: '2026-07-20T10:00:00Z',
    submitted_at: '2026-07-20T10:05:00Z',
    graded_at: '2026-07-20T10:06:00Z',
    ...overrides,
  }
}

/** 课时作答记录页：历史列表、状态/成绩、进专注模式、分页与空态。 */
describe('课时作答记录页', () => {
  it('倒序列出历史作答：状态徽标、成绩与提交时间；无泄露答案字段', async () => {
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/attempts`)

    // 种子：3 条随堂测验作答（进行中/批阅失败×2）+ 1 条已批阅客观题自测
    const rows = await screen.findAllByRole('link', { name: /随堂测验|客观题自测/ })
    expect(rows).toHaveLength(4)
    // started_at 倒序：已批阅的客观题自测（07-16）排在最后
    expect(rows[3]).toHaveTextContent('期望与方差 · 客观题自测')
    expect(rows[3]).toHaveTextContent('已批阅')
    expect(rows[3]).toHaveTextContent('成绩 10 / 15')
    expect(rows[3]).toHaveTextContent('提交于')
    expect(rows[0]).toHaveTextContent('作答中')
    // 列表摘要不渲染任何答案/解析内容
    expect(screen.queryByText('漏选')).not.toBeInTheDocument()
  })

  it('点击已批阅作答进入专注模式，可见 AI 逐题反馈', async () => {
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/attempts`)

    const gradedRow = (await screen.findAllByRole('link', { name: /客观题自测/ }))[0]
    expect(gradedRow).toHaveAttribute('href', `/attempts/${ATTEMPT_GRADED_ID}`)
    fireEvent.click(gradedRow)

    // 专注模式结果视图：测验标题 + 最终成绩 + AI 反馈
    await screen.findByRole('heading', { name: '期望与方差 · 客观题自测' })
    expect(screen.getAllByText('已批阅').length).toBeGreaterThan(0)
    expect(screen.getByText(/最终成绩/)).toBeInTheDocument()
    await screen.findByText(/漏选：泊松分布也是离散型分布。/)
  })

  it('无作答课时显示空态，保留前往测验入口', async () => {
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_BAYES_ID}/attempts`)

    await screen.findByText('本课时还没有作答记录')
    expect(screen.getByRole('link', { name: '前往测验' })).toHaveAttribute(
      'href',
      `/courses/${COURSE_STATS_ID}/lessons/${LESSON_BAYES_ID}/quizzes`,
    )
  })

  it('「加载更多」追加第二页', async () => {
    const page1 = summaryRow({ quiz_title: '第一页作答' })
    const page2 = summaryRow({
      id: 'd0000000-0000-4000-8000-000000000102',
      quiz_title: '第二页作答',
      started_at: '2026-07-19T10:00:00Z',
    })
    server.use(
      http.get(`*/api/v1/lessons/${LESSON_RANDOM_ID}/attempts`, ({ request }) => {
        const url = new URL(request.url)
        const body = url.searchParams.get('cursor')
          ? { items: [page2], page_info: { next_cursor: null, has_more: false } }
          : { items: [page1], page_info: { next_cursor: '2', has_more: true } }
        return HttpResponse.json(body, { headers: echoRequestId(request) })
      }),
    )
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/attempts`)

    await screen.findByText('第一页作答')
    expect(screen.queryByText('第二页作答')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '加载更多' }))
    await screen.findByText('第二页作答')
    expect(screen.queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument()
  })
})
