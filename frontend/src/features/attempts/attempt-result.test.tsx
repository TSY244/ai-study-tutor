import { fireEvent, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import {
  ATTEMPT_FAILED_ID,
  ATTEMPT_FAILED_WITH_JOB_ID,
  JOB_GRADING_FAILED_ID,
} from '@/mocks/fixtures'
import { captureRequests } from '@/test/mock-helpers'
import { startTestJobStream } from '@/test/stream-driver'
import { renderWithRouter } from '@/test/utils'

/**
 * 结果视图集成测试：grading_failed 保留答案/客观分与两条恢复路径
 * （Job Retry / Attempt grading-jobs），以及 graded 反馈渲染。
 * 批阅完成由 SSE 终态事件驱动（精准失效 → REST 重取），不轮询。
 */

describe('grading_failed', () => {
  it('无关联 Job：保留答案与客观分，「重新批阅」走 grading-jobs 并在 SSE 终态后 graded', async () => {
    const gradingJobsCap = captureRequests(
      'POST',
      `/api/v1/attempts/${ATTEMPT_FAILED_ID}/grading-jobs`,
    )
    const { client } = renderWithRouter(`/attempts/${ATTEMPT_FAILED_ID}`)

    // 批阅失败信息 + 答案/客观分保留
    await screen.findByText('AI 批阅失败')
    expect(screen.getByText(/Claude 服务暂时不可用/)).toBeInTheDocument()
    expect(screen.getByText(/客观题暂定分 5 \/ 15/)).toBeInTheDocument()
    expect(screen.getByText('E(X) = np，Var(X) = np(1-p)。')).toBeInTheDocument()
    expect(screen.getByText('B. 掷一枚骰子的点数')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '重新批阅' }))

    // 202 → attempt 进入 grading → SSE 终态事件后 graded
    await screen.findByText(/批阅进行中/, {}, { timeout: 5000 })
    expect(gradingJobsCap.requests).toHaveLength(1)
    expect(gradingJobsCap.requests[0].key).toBeTruthy() // Idempotency-Key

    const stream = startTestJobStream(client)
    stream.settleActiveJob('attempt_grade', ATTEMPT_FAILED_ID)
    await screen.findAllByText('已批阅', {}, { timeout: 5000 }) // 页头与成绩卡各一枚徽章
    expect(screen.getByText(/最终成绩/)).toBeInTheDocument()
    expect(screen.getByText('15 / 15')).toBeInTheDocument()
    // 主观题反馈完整渲染
    expect(screen.getByText('要点完整，表述清晰。')).toBeInTheDocument()
    expect(screen.getByText(/参考答案需覆盖期望与方差两个公式/)).toBeInTheDocument()
    stream.stop()
    gradingJobsCap.stop()
  }, 10000)

  it('有可重试的失败 Job：「重试批阅任务」走 Job Retry 并在 SSE 终态后 graded', async () => {
    const retryCap = captureRequests('POST', `/api/v1/jobs/${JOB_GRADING_FAILED_ID}/retry`)
    const { client } = renderWithRouter(`/attempts/${ATTEMPT_FAILED_WITH_JOB_ID}`)

    await screen.findByText('AI 批阅失败')
    // 有 grading_job_id → 展示任务链接与 Job Retry 入口
    expect(screen.getByRole('link', { name: '查看批阅任务' })).toHaveAttribute(
      'href',
      `/jobs/${JOB_GRADING_FAILED_ID}`,
    )

    fireEvent.click(await screen.findByRole('button', { name: '重试批阅任务' }))

    await screen.findByText(/批阅进行中/, {}, { timeout: 5000 })
    expect(retryCap.requests).toHaveLength(1)
    expect(retryCap.requests[0].key).toBeTruthy()

    const stream = startTestJobStream(client)
    stream.settleActiveJob('attempt_grade', ATTEMPT_FAILED_WITH_JOB_ID)
    await screen.findAllByText('已批阅', {}, { timeout: 5000 }) // 页头与成绩卡各一枚徽章
    stream.stop()
    retryCap.stop()
  }, 10000)
})
