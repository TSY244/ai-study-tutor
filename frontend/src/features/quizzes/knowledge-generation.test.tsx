import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { COURSE_STATS_ID, LESSON_RANDOM_ID } from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { renderWithRouter } from '@/test/utils'

const path = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/quizzes`

describe('知识点测验', () => {
  it.each(['regular', 'review'])(
    '传递 %s 题量和出题备注，失败后保留输入，修改备注换幂等键',
    async (mode) => {
      const user = userEvent.setup()
      const requests: Array<{ body: Record<string, unknown>; key: string | null }> = []
      server.use(
        http.post(
          `*/api/v1/lessons/:id/${mode === 'review' ? 'review-' : ''}quiz-generations`,
          async ({ request }) => {
            requests.push({
              body: (await request.json()) as Record<string, unknown>,
              key: request.headers.get('Idempotency-Key'),
            })
            return HttpResponse.json(
              {
                type: 'about:blank',
                title: '暂不可用',
                status: 503,
                code: 'service_unavailable',
                detail: '请稍后重试。',
              },
              { status: 503 },
            )
          },
        ),
      )
      renderWithRouter(path)
      await user.click(await screen.findByRole('button', { name: '生成测验' }))
      const dialog = screen.getByRole('dialog', { name: '生成知识点测验' })
      await user.selectOptions(within(dialog).getByLabelText('测验类型'), mode)
      fireEvent.change(within(dialog).getByLabelText('题量（可选）'), { target: { value: '5' } })
      const notes = within(dialog).getByLabelText('出题备注（可选）')
      await user.type(notes, '多考边界条件和应用题')
      await user.click(within(dialog).getByRole('button', { name: '开始生成' }))
      await waitFor(() => expect(requests).toHaveLength(1))
      await waitFor(() =>
        expect(within(dialog).getByRole('button', { name: '开始生成' })).toBeEnabled(),
      )
      expect(notes).toHaveValue('多考边界条件和应用题')
      expect(requests[0]?.body).toMatchObject({
        question_count: 5,
        generation_notes: '多考边界条件和应用题',
      })
      await user.type(notes, '，减少记忆题')
      await user.click(within(dialog).getByRole('button', { name: '开始生成' }))
      await waitFor(() => expect(requests).toHaveLength(2))
      expect(requests[1]?.key).not.toEqual(requests[0]?.key)
    },
  )

  it('展示知识点解释、学习目标和原文依据', async () => {
    server.use(
      http.get('*/api/v1/courses/:id/knowledge-points', () =>
        HttpResponse.json({
          items: [
            {
              id: 'point-1',
              course_id: COURSE_STATS_ID,
              lesson_id: LESSON_RANDOM_ID,
              lesson_title: '随机变量',
              label: '有序性',
              importance: 'core',
              explanation: '二分查找依赖有序性。',
              learning_objective: '判断二分查找的适用前提。',
              source_excerpt: '数组必须有序',
              updated_at: new Date().toISOString(),
            },
          ],
        }),
      ),
    )
    renderWithRouter(path)
    const region = await screen.findByRole('region', { name: '本课时知识点' })
    await userEvent.click(await within(region).findByText('有序性'))
    expect(within(region).getByText('二分查找依赖有序性。')).toBeVisible()
    expect(within(region).getByText(/判断二分查找的适用前提/)).toBeVisible()
    expect(within(region).getByText(/原文依据：数组必须有序/)).toBeVisible()
  })
})
