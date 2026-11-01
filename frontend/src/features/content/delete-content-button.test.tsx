import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'

import { DeleteContentButton } from '@/features/content/delete-content-button'
import { SummaryPanel } from '@/features/summaries/summary-panel'
import { loadLastEvaluationId, saveLastEvaluationId } from '@/features/summaries/last-evaluation'
import { COURSE_STATS_ID, LESSON_RANDOM_ID, QUIZ_DEMO_ID, demoDocuments } from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { problemResponse } from '@/test/mock-helpers'
import { renderWithClient, renderWithRouter } from '@/test/utils'

async function confirmDelete(user: ReturnType<typeof userEvent.setup>) {
  const dialog = await screen.findByRole('dialog')
  await user.click(within(dialog).getByRole('button', { name: '删除' }))
}

describe('可见的单项删除', () => {
  it('课程列表直接显示删除；取消不发请求，确认才删除并刷新列表', async () => {
    const user = userEvent.setup()
    const remove = vi.fn()
    server.use(
      http.delete(`*/api/v1/courses/${COURSE_STATS_ID}`, () => {
        remove()
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const { client } = renderWithRouter('/courses')
    const invalidate = vi.spyOn(client, 'invalidateQueries')
    await user.click(await screen.findByRole('button', { name: '删除课程「统计学基础」' }))
    expect(remove).not.toHaveBeenCalled()
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '取消' }))
    expect(remove).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: '删除课程「统计学基础」' }))
    await confirmDelete(user)
    await waitFor(() => expect(remove).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith(expect.objectContaining({ queryKey: ['courses'] })),
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('级联确认范围清楚，删除测验后刷新作答与学习进度', async () => {
    const user = userEvent.setup()
    const remove = vi.fn()
    server.use(
      http.delete(`*/api/v1/quizzes/${QUIZ_DEMO_ID}`, () => {
        remove()
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const { client } = renderWithClient(
      <DeleteContentButton kind="quiz" id={QUIZ_DEMO_ID} name="练习" lessonId={LESSON_RANDOM_ID} />,
    )
    for (const key of [
      ['lesson-attempts', LESSON_RANDOM_ID],
      ['progress', COURSE_STATS_ID],
      ['weaknesses', {}],
      ['system', 'status'],
    ])
      client.setQueryData(key, { seeded: true })
    await user.click(screen.getByRole('button', { name: '删除测验「练习」' }))
    expect(
      within(screen.getByRole('dialog')).getByText(/作答记录、答案、评分和相关进度快照将一并删除/),
    ).toBeInTheDocument()
    await confirmDelete(user)
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(remove).toHaveBeenCalledTimes(1)
    expect(client.getQueryState(['lesson-attempts', LESSON_RANDOM_ID])?.isInvalidated).toBe(true)
    expect(client.getQueryState(['progress', COURSE_STATS_ID])?.isInvalidated).toBe(true)
    expect(client.getQueryState(['weaknesses', {}])?.isInvalidated).toBe(true)
    expect(client.getQueryState(['system', 'status'])?.isInvalidated).toBe(false)
  })

  it('活动任务导致 409 时保留确认框与资源，提示完成或取消后重试', async () => {
    const user = userEvent.setup()
    server.use(
      http.delete(`*/api/v1/quizzes/${QUIZ_DEMO_ID}`, () =>
        problemResponse({ status: 409, code: 'resource_busy' }),
      ),
    )
    const deleted = vi.fn()
    renderWithClient(
      <DeleteContentButton
        kind="quiz"
        id={QUIZ_DEMO_ID}
        name="练习"
        lessonId={LESSON_RANDOM_ID}
        onDeleted={deleted}
      />,
    )
    await user.click(screen.getByRole('button', { name: '删除测验「练习」' }))
    await confirmDelete(user)
    expect(await screen.findByRole('alert')).toHaveTextContent('请等待任务完成或取消后再删除')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(deleted).not.toHaveBeenCalled()
  })

  it.each([
    ['quiz', 'quizzes'],
    ['knowledge', 'knowledge-points'],
    ['document', 'documents'],
    ['material', 'materials'],
    ['attempt', 'attempts'],
    ['weakness', 'weaknesses'],
  ] as const)('%s 删除成功后调用对应 API 并通知页面', async (kind, endpoint) => {
    const user = userEvent.setup()
    const remove = vi.fn()
    const deleted = vi.fn()
    server.use(
      http.delete(`*/api/v1/${endpoint}/item-id`, () => {
        remove()
        return new HttpResponse(null, { status: 204 })
      }),
    )
    renderWithClient(
      <DeleteContentButton
        kind={kind}
        id="item-id"
        name="测试"
        courseId={COURSE_STATS_ID}
        lessonId={LESSON_RANDOM_ID}
        onDeleted={deleted}
      />,
    )
    await user.click(screen.getByRole('button', { name: /删除.+「测试」/ }))
    await confirmDelete(user)
    await waitFor(() => expect(deleted).toHaveBeenCalledTimes(1))
    expect(remove).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('删除当前笔记后显示空状态，仍可查看和删除其他历史版本', async () => {
    const user = userEvent.setup()
    const notes = demoDocuments.filter(
      (doc) => doc.lesson_id === LESSON_RANDOM_ID && doc.kind === 'student_notes',
    )
    const current = notes.find((doc) => doc.is_active)!
    let removed = false
    server.use(
      http.delete(`*/api/v1/documents/${current.id}`, () => {
        removed = true
        return new HttpResponse(null, { status: 204 })
      }),
      http.get(`*/api/v1/lessons/${LESSON_RANDOM_ID}/documents/student_notes`, () =>
        removed ? problemResponse({ status: 404, code: 'not_found' }) : HttpResponse.json(current),
      ),
      http.get(`*/api/v1/lessons/${LESSON_RANDOM_ID}/documents/student_notes/versions`, () =>
        HttpResponse.json({
          items: notes.filter((doc) => !removed || doc.id !== current.id),
          page_info: { has_more: false, next_cursor: null },
        }),
      ),
    )
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/notes`)
    await user.click(await screen.findByRole('button', { name: /删除文档/ }))
    await confirmDelete(user)
    expect(await screen.findByText('笔记尚未生成')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '历史版本' }))
    const dialog = await screen.findByRole('dialog')
    const r1 = await within(dialog).findByText('R1')
    await user.click(within(r1.closest('li')!).getByRole('button', { name: '查看' }))
    expect(await screen.findByText(/正在查看历史版本 R1/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /删除文档.*R1/ })).toBeInTheDocument()
  })

  it('总结历史来自服务端；删除当前总结清除选择并保留输入草稿', async () => {
    const user = userEvent.setup()
    const evaluation = {
      id: 'eval-delete',
      lesson_id: LESSON_RANDOM_ID,
      summary_text: '这是一份旧总结',
      status: 'succeeded',
      created_at: '2026-10-01T00:00:00Z',
      correct_points: [],
      misconceptions: [],
      missing_points: [],
      suggestions: [],
    }
    let deleted = false
    server.use(
      http.get('*/api/v1/lessons/:lessonId/summary-evaluations', () =>
        HttpResponse.json({
          items: deleted ? [] : [evaluation],
          page_info: { has_more: false, next_cursor: null },
        }),
      ),
      http.get('*/api/v1/summary-evaluations/eval-delete', () => HttpResponse.json(evaluation)),
      http.delete('*/api/v1/summary-evaluations/eval-delete', () => {
        deleted = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
    saveLastEvaluationId(LESSON_RANDOM_ID, evaluation.id)
    renderWithClient(<SummaryPanel lessonId={LESSON_RANDOM_ID} />)
    expect(await screen.findByText('这是一份旧总结')).toBeInTheDocument()
    await user.type(screen.getByLabelText('总结内容'), '还没提交的新草稿')
    await user.click(screen.getByRole('button', { name: /删除总结/ }))
    await confirmDelete(user)
    await waitFor(() => expect(screen.queryByText('这是一份旧总结')).not.toBeInTheDocument())
    expect(loadLastEvaluationId(LESSON_RANDOM_ID)).toBeNull()
    expect(screen.getByLabelText('总结内容')).toHaveValue('还没提交的新草稿')
  })
})
