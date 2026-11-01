import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import {
  loadLastEvaluationId,
  resetLastEvaluationIds,
  saveLastEvaluationId,
} from '@/features/summaries/last-evaluation'
import { COURSE_STATS_ID, LESSON_RANDOM_ID } from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { renderWithRouter } from '@/test/utils'

const SUMMARY_PATH = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/summary`

describe('最近评估 id 记录', () => {
  it('按课时存取；不存在时返回 null', () => {
    expect(loadLastEvaluationId(LESSON_RANDOM_ID)).toBeNull()
    saveLastEvaluationId(LESSON_RANDOM_ID, 'eval-1')
    expect(loadLastEvaluationId(LESSON_RANDOM_ID)).toBe('eval-1')
    resetLastEvaluationIds()
    expect(loadLastEvaluationId(LESSON_RANDOM_ID)).toBeNull()
  })
})

describe('学习总结', () => {
  it('未提交过时展示说明，空内容不可提交', async () => {
    renderWithRouter(SUMMARY_PATH)

    expect(
      await screen.findByText(
        '还没有提交过总结。提交后这里会显示 AI 的覆盖度评分、误解与遗漏分析。',
      ),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '提交评估' })).toBeDisabled()
  })

  it('提交总结后经历「进行中」到展示覆盖度与四类反馈', async () => {
    const user = userEvent.setup()
    const { client } = renderWithRouter(SUMMARY_PATH)

    const textarea = await screen.findByLabelText('总结内容')
    await user.type(textarea, '随机变量分为离散与连续两类，期望具有线性性质。')
    await user.click(screen.getByRole('button', { name: '提交评估' }))

    // 202 接受后先进入「评估进行中」
    expect(await screen.findByText('AI 评估进行中…')).toBeInTheDocument()
    const jobs = await screen.findByRole('region', { name: '总结正在评估' })
    expect(within(jobs).getByRole('progressbar')).toBeInTheDocument()
    expect(within(jobs).getByRole('link', { name: '查看任务详情' })).toBeInTheDocument()

    // 模拟 SSE 终态对账：失效评估查询 → 重取时 MSW 推进为完成
    await client.invalidateQueries({ queryKey: ['summary-evaluation'] })

    expect(await screen.findByText('评估完成')).toBeInTheDocument()
    expect(screen.getByText('要点覆盖度')).toBeInTheDocument()
    expect(screen.getByText('75%')).toBeInTheDocument()
    expect(screen.getByText('已掌握的要点')).toBeInTheDocument()
    expect(screen.getByText('存在的误解')).toBeInTheDocument()
    expect(screen.getByText('遗漏的要点')).toBeInTheDocument()
    expect(screen.getByText('学习建议')).toBeInTheDocument()
    expect(
      screen.getByText('把「方差」理解成了数据的平均偏差，实际是绝对偏差的平方期望。'),
    ).toBeInTheDocument()
  })

  it('评估失败时展示错误码并提示重新提交', async () => {
    const user = userEvent.setup()
    const { client } = renderWithRouter(SUMMARY_PATH)

    const textarea = await screen.findByLabelText('总结内容')
    await user.type(textarea, '一段会触发评估失败的总结。')
    await user.click(screen.getByRole('button', { name: '提交评估' }))
    expect(await screen.findByText('AI 评估进行中…')).toBeInTheDocument()

    // Worker 失败：GET 返回 failed + error
    server.use(
      http.get('*/api/v1/summary-evaluations/:evaluationId', ({ params, request }) =>
        HttpResponse.json(
          {
            id: params.evaluationId,
            lesson_id: LESSON_RANDOM_ID,
            status: 'failed',
            coverage_score: null,
            correct_points: [],
            misconceptions: [],
            missing_points: [],
            suggestions: [],
            error: {
              code: 'claude_unavailable',
              detail: 'Claude 服务暂时不可用，请稍后重试。',
              retryable: true,
            },
            created_at: new Date().toISOString(),
            completed_at: new Date().toISOString(),
          },
          {
            status: 200,
            headers: { 'X-Request-ID': request.headers.get('X-Request-ID') ?? 'mock-request-id' },
          },
        ),
      ),
    )

    // 触发重取（等价于 SSE 终态失效）
    await client.invalidateQueries({ queryKey: ['summary-evaluation'] })

    expect(await screen.findByText('AI 评估失败')).toBeInTheDocument()
    expect(screen.getByText(/Claude 服务暂时不可用/)).toBeInTheDocument()
    expect(screen.getByText(/claude_unavailable/)).toBeInTheDocument()
  })
})
