import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { COURSE_LINALG_ID, COURSE_STATS_ID } from '@/mocks/fixtures'
import { server } from '@/mocks/server'
import { captureRequests, problemResponse } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

/** 课程操作：编辑（对话框表单）、归档/恢复、删除（确认框 + 导航）。 */
describe('CourseActions', () => {
  it('编辑课程：对话框修改标题，PATCH 带 version，成功后标题更新', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('PATCH', `/api/v1/courses/${COURSE_STATS_ID}`)
    renderWithRouter(`/courses/${COURSE_STATS_ID}`)

    await user.click(await screen.findByRole('button', { name: '编辑' }))
    const dialog = await screen.findByRole('dialog')
    const titleInput = within(dialog).getByLabelText(/课程标题/)
    await user.clear(titleInput)
    await user.type(titleInput, '统计学基础（第二版）')
    await user.click(within(dialog).getByRole('button', { name: '保存' }))

    await waitFor(() =>
      expect(screen.getByRole('heading', { name: '统计学基础（第二版）' })).toBeInTheDocument(),
    )
    expect(requests).toHaveLength(1)
    expect(requests[0]?.body).toMatchObject({ title: '统计学基础（第二版）', version: 4 })
    stop()
  })

  it('归档课程：确认后 PATCH archived=true，徽标出现', async () => {
    const user = userEvent.setup()
    renderWithRouter(`/courses/${COURSE_LINALG_ID}`)

    await user.click(await screen.findByRole('button', { name: '归档' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: '归档' }))

    expect(await screen.findByText('已归档')).toBeInTheDocument()
    // 操作切换为“恢复”
    expect(screen.getByRole('button', { name: '恢复' })).toBeInTheDocument()
  })

  it('删除课程：确认后 DELETE，回到列表且课程消失', async () => {
    const user = userEvent.setup()
    const { router } = renderWithRouter(`/courses/${COURSE_LINALG_ID}`)

    await user.click(await screen.findByRole('button', { name: '删除' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('不可恢复')
    await user.click(within(dialog).getByRole('button', { name: '删除' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/courses'))
    await waitFor(() =>
      expect(screen.queryByRole('link', { name: /线性代数/ })).not.toBeInTheDocument(),
    )
    expect(screen.getByRole('link', { name: /统计学基础/ })).toBeInTheDocument()
  })

  it('version_conflict：对话框保留，错误按映射表呈现且输入保留', async () => {
    const user = userEvent.setup()
    server.use(
      http.patch(`*/api/v1/courses/:courseId`, () =>
        problemResponse({
          status: 409,
          code: 'version_conflict',
          detail: '课程已被其他操作修改。',
        }),
      ),
    )
    renderWithRouter(`/courses/${COURSE_STATS_ID}`)

    await user.click(await screen.findByRole('button', { name: '编辑' }))
    const dialog = await screen.findByRole('dialog')
    const titleInput = within(dialog).getByLabelText(/课程标题/)
    await user.clear(titleInput)
    await user.type(titleInput, '冲突标题')
    await user.click(within(dialog).getByRole('button', { name: '保存' }))

    const alert = await within(dialog).findByRole('alert')
    expect(alert).toHaveTextContent('版本冲突')
    expect(alert).toHaveTextContent('req-test-1')
    expect(titleInput).toHaveValue('冲突标题')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('课程列表游标分页：加载更多追加下一页', async () => {
    const user = userEvent.setup()
    const manyCourses = Array.from({ length: 25 }, (_, index) => ({
      id: `00000000-0000-4000-8000-${String(index + 1).padStart(12, '0')}`,
      title: `分页课程 ${index + 1}`,
      description: '',
      archived: false,
      lesson_count: 0,
      version: 1,
      created_at: '2026-07-01T00:00:00Z',
      updated_at: '2026-07-01T00:00:00Z',
    }))
    server.use(
      http.get('*/api/v1/courses', ({ request }) => {
        const url = new URL(request.url)
        const limit = Number(url.searchParams.get('limit')) || 20
        const offset = Number(url.searchParams.get('cursor')) || 0
        const items = manyCourses.slice(offset, offset + limit)
        const next = offset + limit
        return HttpResponse.json(
          {
            items,
            page_info: {
              next_cursor: next < manyCourses.length ? String(next) : null,
              has_more: next < manyCourses.length,
            },
          },
          { headers: { 'X-Request-ID': 'req-page' } },
        )
      }),
    )
    renderWithRouter('/courses')

    expect(await screen.findByText('分页课程 1')).toBeInTheDocument()
    expect(screen.queryByText('分页课程 25')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '加载更多' }))
    expect(await screen.findByText('分页课程 25')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '加载更多' })).not.toBeInTheDocument()
  })
})
