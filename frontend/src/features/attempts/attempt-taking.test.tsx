import { fireEvent, screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { createAttempt } from '@/api/generated/endpoints/attempts/attempts'
import type { SavedAnswer } from '@/api/generated/model'
import { server } from '@/mocks/server'
import { ATTEMPT_DEMO_ID, Q_DEMO_2_ID, QUIZ_DEMO_ID, QUIZ_OBJECTIVE_ID } from '@/mocks/fixtures'
import { captureRequests } from '@/test/mock-helpers'
import { startTestJobStream } from '@/test/stream-driver'
import { renderWithRouter } from '@/test/utils'

import { ATTEMPT_DRAFT_STORAGE_PREFIX, loadAttemptDraft } from './attempt-draft'

/**
 * 作答流程集成测试：真实路由 + MSW（含服务端版本语义）。
 * debounce 为真实 800ms，断言用 waitFor 等待。
 */

function answerPatchPath(attemptId: string, questionId: string) {
  return `/api/v1/attempts/${attemptId}/answers/${questionId}`
}

describe('自动保存', () => {
  it('输入立即写入 sessionStorage，debounce 后 PATCH（answer_version=0），成功后显示已同步', async () => {
    const cap = captureRequests('PATCH', answerPatchPath(ATTEMPT_DEMO_ID, Q_DEMO_2_ID))
    renderWithRouter(`/attempts/${ATTEMPT_DEMO_ID}`)

    const textarea = await screen.findByLabelText('作答内容')
    fireEvent.change(textarea, { target: { value: 'E(X)=np，Var(X)=np(1-p)。' } })

    // 立即持久化（不等 debounce）
    const draft = loadAttemptDraft(ATTEMPT_DEMO_ID)
    expect(draft?.answers[Q_DEMO_2_ID]).toMatchObject({
      value: 'E(X)=np，Var(X)=np(1-p)。',
      savedVersion: 0,
      dirty: true,
    })

    // 800ms debounce 后发出 PATCH
    await waitFor(() => expect(cap.requests).toHaveLength(1), { timeout: 3000 })
    await waitFor(() => expect(cap.requests[0].body).not.toBeNull())
    expect(cap.requests[0].body).toMatchObject({
      answer: { value: 'E(X)=np，Var(X)=np(1-p)。' },
      answer_version: 0,
    })
    expect(cap.requests[0].body).toHaveProperty('client_updated_at')

    await screen.findByText('所有答案已同步', {}, { timeout: 3000 })
    cap.stop()
  })

  it('刷新恢复：草稿值回填并自动重发同步', async () => {
    sessionStorage.setItem(
      ATTEMPT_DRAFT_STORAGE_PREFIX + ATTEMPT_DEMO_ID,
      JSON.stringify({
        version: 1,
        attemptId: ATTEMPT_DEMO_ID,
        attemptVersion: 1,
        answers: {
          [Q_DEMO_2_ID]: {
            value: '本地未同步的作答',
            savedVersion: 0,
            dirty: true,
            clientUpdatedAt: '2026-07-19T09:00:00.000Z',
          },
        },
        updatedAt: '2026-07-19T09:00:00.000Z',
      }),
    )
    const cap = captureRequests('PATCH', answerPatchPath(ATTEMPT_DEMO_ID, Q_DEMO_2_ID))
    renderWithRouter(`/attempts/${ATTEMPT_DEMO_ID}`)

    const textarea = (await screen.findByLabelText('作答内容')) as HTMLTextAreaElement
    expect(textarea).toHaveValue('本地未同步的作答')

    await waitFor(() => expect(cap.requests).toHaveLength(1), { timeout: 3000 })
    await screen.findByText('所有答案已同步', {}, { timeout: 3000 })
    cap.stop()
  })

  it('保存失败显示「未同步」，答案保留，恢复后可重试成功', async () => {
    let failing = true
    server.use(
      http.patch('*/api/v1/attempts/:attemptId/answers/:questionId', () => {
        // 返回 undefined 即放行到 curated handler（恢复后走正常版本语义）
        return failing ? HttpResponse.error() : undefined
      }),
    )
    renderWithRouter(`/attempts/${ATTEMPT_DEMO_ID}`)

    const textarea = (await screen.findByLabelText('作答内容')) as HTMLTextAreaElement
    fireEvent.change(textarea, { target: { value: '断网时写的答案' } })

    await screen.findByText(/未同步：答案已保存在本机/, {}, { timeout: 3000 })
    expect(textarea).toHaveValue('断网时写的答案') // 不清空输入

    failing = false
    fireEvent.click(screen.getByRole('button', { name: /立即重试/ }))
    await screen.findByText('所有答案已同步', {}, { timeout: 3000 })
  })
})

describe('409 冲突处理', () => {
  /** answer_version=0 的首次保存一律 409（带 current_answer 扩展），之后正常保存。 */
  function useConflictHandler(currentValue: unknown) {
    server.use(
      http.patch(
        '*/api/v1/attempts/:attemptId/answers/:questionId',
        async ({ request, params }) => {
          const body = (await request.json()) as {
            answer: { value: unknown }
            answer_version: number
          }
          if (body.answer_version === 0) {
            return HttpResponse.json(
              {
                type: 'about:blank',
                title: '答案版本冲突',
                status: 409,
                detail: '服务端已有更新的答案。',
                code: 'version_conflict',
                current_answer: { value: currentValue },
                current_answer_version: 2,
                current_attempt_version: 9,
              },
              {
                status: 409,
                headers: {
                  'Content-Type': 'application/problem+json',
                  'X-Request-ID': 'req-conflict',
                },
              },
            )
          }
          const saved: SavedAnswer = {
            attempt_id: params.attemptId as string,
            question_id: params.questionId as string,
            answer: body.answer,
            answer_version: body.answer_version + 1,
            attempt_version: 10,
            saved_at: new Date().toISOString(),
          }
          return HttpResponse.json(saved, { headers: { 'X-Request-ID': 'req-save' } })
        },
      ),
    )
  }

  it('无法安全合并时展示冲突对话框，「保留我的输入」用服务端版本重发', async () => {
    useConflictHandler('服务端已有答案')
    const cap = captureRequests('PATCH', answerPatchPath(ATTEMPT_DEMO_ID, Q_DEMO_2_ID))
    renderWithRouter(`/attempts/${ATTEMPT_DEMO_ID}`)

    const textarea = (await screen.findByLabelText('作答内容')) as HTMLTextAreaElement
    fireEvent.change(textarea, { target: { value: '我的本地答案' } })

    const dialog = await screen.findByRole('dialog', {}, { timeout: 3000 })
    expect(dialog).toHaveTextContent('我的本地答案')
    expect(dialog).toHaveTextContent('服务端已有答案')

    fireEvent.click(screen.getByRole('button', { name: '保留我的输入' }))
    // 以服务端 answer_version=2 为基线重发本地值
    await waitFor(() => expect(cap.requests).toHaveLength(2), { timeout: 3000 })
    await waitFor(() => expect(cap.requests[1].body).not.toBeNull())
    expect(cap.requests[1].body).toMatchObject({
      answer: { value: '我的本地答案' },
      answer_version: 2,
    })
    await screen.findByText('所有答案已同步', {}, { timeout: 3000 })
    cap.stop()
  })

  it('「使用已保存版本」放弃本地修改并回填服务端值', async () => {
    useConflictHandler('服务端已有答案')
    const cap = captureRequests('PATCH', answerPatchPath(ATTEMPT_DEMO_ID, Q_DEMO_2_ID))
    renderWithRouter(`/attempts/${ATTEMPT_DEMO_ID}`)

    const textarea = (await screen.findByLabelText('作答内容')) as HTMLTextAreaElement
    fireEvent.change(textarea, { target: { value: '我的本地答案' } })

    await screen.findByRole('dialog', {}, { timeout: 3000 })
    fireEvent.click(screen.getByRole('button', { name: '使用已保存版本' }))

    await waitFor(() => expect(textarea).toHaveValue('服务端已有答案'), { timeout: 3000 })
    expect(cap.requests).toHaveLength(1) // 不再重发
    await screen.findByText('所有答案已同步', {}, { timeout: 3000 })
    cap.stop()
  })
})

describe('提交', () => {
  it('全客观题：提交前自动 flush 未保存答案，200 分支直接出成绩（无 Job）', async () => {
    const attempt = await createAttempt(QUIZ_OBJECTIVE_ID)
    const submitCap = captureRequests('POST', `/api/v1/attempts/${attempt.id}/submit`)
    renderWithRouter(`/attempts/${attempt.id}`)

    // 立即点提交（不等 debounce）：提交前必须 flush 三题
    fireEvent.click(await screen.findByRole('radio', { name: /B\. E\(X\) 是/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: /A\. 二项分布/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: /B\. 泊松分布/ }))
    fireEvent.click(screen.getByRole('radio', { name: '正确' }))
    fireEvent.click(screen.getByRole('button', { name: '提交作答' }))
    fireEvent.click(await screen.findByRole('button', { name: '确认提交' }))

    // 200 AttemptSubmitCompleted：直接 graded，无批阅等待
    await screen.findByText(/最终成绩/, {}, { timeout: 5000 })
    expect(screen.queryByText(/批阅进行中/)).not.toBeInTheDocument()

    await waitFor(() => expect(submitCap.requests).toHaveLength(1))
    await waitFor(() => expect(submitCap.requests[0].body).not.toBeNull())
    // 初始 version=1 + 3 次自动保存 → 提交 version=4
    expect(submitCap.requests[0].body).toEqual({ version: 4 })
    expect(submitCap.requests[0].key).toBeTruthy() // Idempotency-Key

    // 全对：15 / 15，且提交后无输入控件（锁定）
    await screen.findByText('15 / 15', {}, { timeout: 3000 })
    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    submitCap.stop()
  })

  it('含主观题：202 分支显示批阅进行中，SSE 终态事件后出最终成绩与逐题反馈', async () => {
    const attempt = await createAttempt(QUIZ_DEMO_ID)
    const { client } = renderWithRouter(`/attempts/${attempt.id}`)

    fireEvent.click(await screen.findByRole('radio', { name: /B\. 掷一枚骰子的点数/ }))
    fireEvent.change(await screen.findByLabelText('作答内容'), {
      target: { value: 'E(X)=np，Var(X)=np(1-p)。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '提交作答' }))
    fireEvent.click(await screen.findByRole('button', { name: '确认提交' }))

    // 202 AttemptSubmitAccepted：先有批阅进行中 + 客观题暂定分
    await screen.findByText(/AI 批阅进行中/, {}, { timeout: 5000 })
    expect(screen.getByText(/客观题暂定分 5 \/ 15/)).toBeInTheDocument()

    // SSE：批阅任务 job.succeeded → attempt 精准失效 → REST 重取返回 graded
    const stream = startTestJobStream(client)
    stream.settleActiveJob('attempt_grade', attempt.id)
    await screen.findAllByText('已批阅', {}, { timeout: 5000 }) // 页头与成绩卡各一枚徽章
    expect(screen.getByText(/最终成绩/)).toBeInTheDocument()
    expect(screen.getByText('15 / 15')).toBeInTheDocument()
    expect(screen.getByText('要点完整，表述清晰。')).toBeInTheDocument()
    expect(screen.getAllByText('正确').length).toBeGreaterThan(0)
    stream.stop()
  }, 10000)

  it('提交前若仍有未同步（网络失败），阻止提交并保留答案', async () => {
    server.use(
      http.patch('*/api/v1/attempts/:attemptId/answers/:questionId', () => HttpResponse.error()),
    )
    renderWithRouter(`/attempts/${ATTEMPT_DEMO_ID}`)

    const textarea = (await screen.findByLabelText('作答内容')) as HTMLTextAreaElement
    fireEvent.change(textarea, { target: { value: '同步不了的答案' } })
    fireEvent.click(screen.getByRole('button', { name: '提交作答' }))
    fireEvent.click(await screen.findByRole('button', { name: '确认提交' }))

    await screen.findByText(/仍有答案未同步或存在冲突/, {}, { timeout: 5000 })
    expect(textarea).toHaveValue('同步不了的答案') // 作答界面仍在，答案未丢
  }, 10000)
})
