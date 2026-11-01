import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import {
  COURSE_LINALG_ID,
  COURSE_STATS_ID,
  LESSON_CONFIDENCE_ID,
  LESSON_RANDOM_ID,
  LESSON_VECTOR_ID,
  demoCourses,
  demoLessons,
} from '@/mocks/fixtures'
import { queryKeys } from '@/api/query-client'
import { server } from '@/mocks/server'
import { captureRequests, problemResponse } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

/** 列表页多选删除：选择模式 -> 勾选/全选 -> 确认框 -> 原子批量删除 -> 列表刷新。 */
describe('列表批量删除', () => {
  it('切换课程后清空选择，删除只提交当前课程的课时', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', '/api/v1/lessons:batch-delete')
    const { router, client } = renderWithRouter(`/courses/${COURSE_STATS_ID}`)
    client.setQueryData(queryKeys.course(COURSE_LINALG_ID), demoCourses[1])
    await screen.findByRole('link', { name: /随机变量与分布/ })
    await user.click(screen.getByRole('button', { name: '选择' }))
    await user.click(screen.getByRole('checkbox', { name: '选择 随机变量与分布' }))

    await act(async () => {
      await router.navigate({
        to: '/courses/$courseId',
        params: { courseId: COURSE_LINALG_ID },
      })
    })

    await screen.findByRole('heading', { name: '线性代数', level: 1 })
    await screen.findByText('向量与线性组合')
    expect(screen.queryByText('已选 1 项')).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '选择' }))
    await user.click(screen.getByRole('checkbox', { name: '选择 向量与线性组合' }))
    await user.click(screen.getByRole('button', { name: '删除所选' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: '删除' }))
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]?.body).toEqual({ ids: [LESSON_VECTOR_ID] })
    stop()
  })

  it.each(['课程', '课时'] as const)(
    '%s列表超过 100 项时阻止删除，减少到 100 项后可继续',
    async (kind) => {
      const user = userEvent.setup()
      const courses = Array.from({ length: 101 }, (_, index) => ({
        ...demoCourses[0]!,
        id: `00000000-0000-4000-8000-${String(index).padStart(12, '0')}`,
        title: `边界条目 ${index}`,
      }))
      const lessons = courses.map((course, index) => ({
        ...demoLessons[0]!,
        id: course.id,
        title: course.title,
        ordinal: index + 1,
      }))
      const path = kind === '课程' ? '*/api/v1/courses' : '*/api/v1/courses/:courseId/lessons'
      const items = kind === '课程' ? courses : lessons
      server.use(
        http.get(path, ({ request }) => {
          const params = new URL(request.url).searchParams
          const offset = Number(params.get('cursor') ?? 0)
          const limit = Number(params.get('limit'))
          const next = offset + limit
          return HttpResponse.json({
            items: items.slice(offset, next),
            page_info: {
              has_more: next < items.length,
              next_cursor: next < items.length ? String(next) : null,
            },
          })
        }),
      )
      renderWithRouter(kind === '课程' ? '/courses' : `/courses/${COURSE_STATS_ID}`)
      await screen.findByRole('link', { name: /边界条目 0\b/ })
      const pageSize = kind === '课程' ? 20 : 50
      for (let offset = pageSize; offset < 101; offset += pageSize) {
        await user.click(screen.getByRole('button', { name: '加载更多' }))
        await screen.findByRole('link', {
          name: new RegExp(`边界条目 ${Math.min(offset + pageSize - 1, 100)}\\b`),
        })
      }
      await user.click(screen.getByRole('button', { name: '选择' }))
      await user.click(screen.getByRole('checkbox', { name: '全选' }))
      expect(screen.getByText('已选 101 项')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '删除所选' })).toBeDisabled()
      expect(screen.getByRole('alert')).toHaveTextContent('一次最多删除 100 项')
      await user.click(screen.getByRole('button', { name: '删除所选' }))
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

      await user.click(screen.getByRole('checkbox', { name: '选择 边界条目 100' }))
      expect(screen.getByText('已选 100 项')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '删除所选' })).toBeEnabled()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: '删除所选' }))
      expect(await screen.findByRole('dialog')).toHaveTextContent('所选 100')
    },
    // 101 行的可访问性查询在构建容器中较慢，保留完整边界断言。
    30000,
  )

  it('课程列表：勾选后全选，确认删除调用 batch-delete 且列表刷新、退出选择模式', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', '/api/v1/courses:batch-delete')
    const refreshes = captureRequests('GET', '/api/v1/courses')
    const { client } = renderWithRouter('/courses')

    expect(await screen.findByRole('link', { name: /统计学基础/ })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '选择' }))

    // 行变为 checkbox（不再跳转）；单选一门后全选呈半选态
    await user.click(screen.getByRole('checkbox', { name: '选择 统计学基础' }))
    expect(screen.getByText('已选 1 项')).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: '全选' })).toBePartiallyChecked()

    // 无选中时禁用（先取消勾选验证），再全选两门
    await user.click(screen.getByRole('checkbox', { name: '选择 统计学基础' }))
    expect(screen.getByRole('button', { name: '删除所选' })).toBeDisabled()
    await user.click(screen.getByRole('checkbox', { name: '全选' }))
    expect(screen.getByText('已选 2 项')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '删除所选' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('将删除所选 2 门课程')
    await waitFor(() => expect(client.isFetching()).toBe(0))
    refreshes.requests.length = 0
    await user.click(within(dialog).getByRole('button', { name: '删除' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]?.body).toEqual({
      ids: expect.arrayContaining([COURSE_STATS_ID, COURSE_LINALG_ID]),
    })
    await waitFor(() =>
      expect(screen.queryByRole('link', { name: /统计学基础/ })).not.toBeInTheDocument(),
    )
    // 成功后退出选择模式
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(refreshes.requests).toHaveLength(1)
    refreshes.stop()
    stop()
  })

  it('课时列表：勾选两个课时，确认删除调用 batch-delete，未选课时保留', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', '/api/v1/lessons:batch-delete')
    const refreshes = captureRequests('GET', `/api/v1/courses/${COURSE_STATS_ID}/lessons`)
    const { client } = renderWithRouter(`/courses/${COURSE_STATS_ID}`)

    expect(await screen.findByRole('link', { name: /随机变量与分布/ })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '选择' }))

    await user.click(screen.getByRole('checkbox', { name: '选择 随机变量与分布' }))
    await user.click(screen.getByRole('checkbox', { name: '选择 置信区间' }))
    expect(screen.getByText('已选 2 项')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '删除所选' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('将删除所选 2 个课时')
    await waitFor(() => expect(client.isFetching()).toBe(0))
    refreshes.requests.length = 0
    await user.click(within(dialog).getByRole('button', { name: '删除' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]?.body).toEqual({
      ids: expect.arrayContaining([LESSON_RANDOM_ID, LESSON_CONFIDENCE_ID]),
    })
    await waitFor(() => expect(screen.queryByText('随机变量与分布')).not.toBeInTheDocument())
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(screen.getByRole('link', { name: /贝叶斯定理/ })).toBeInTheDocument()
    expect(refreshes.requests).toHaveLength(1)
    refreshes.stop()
    stop()
  })

  it('取消选择：清空勾选并恢复整行跳转', async () => {
    const user = userEvent.setup()
    renderWithRouter('/courses')

    expect(await screen.findByRole('link', { name: /统计学基础/ })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '选择' }))
    await user.click(screen.getByRole('checkbox', { name: '选择 统计学基础' }))
    await user.click(screen.getByRole('button', { name: '取消' }))

    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: /统计学基础/ })).toBeInTheDocument()
  })

  it('resource_busy（409）：错误留在确认框内，列表不刷新、选中保留', async () => {
    const user = userEvent.setup()
    server.use(
      http.post('*/api/v1/lessons\\:batch-delete', () =>
        problemResponse({
          status: 409,
          code: 'resource_busy',
          title: '已有任务在进行',
          detail: '贝叶斯定理有进行中的生成任务，整体未删除。',
        }),
      ),
    )
    renderWithRouter(`/courses/${COURSE_STATS_ID}`)

    expect(await screen.findByRole('link', { name: /贝叶斯定理/ })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '选择' }))
    await user.click(screen.getByRole('checkbox', { name: '选择 贝叶斯定理' }))
    await user.click(screen.getByRole('button', { name: '删除所选' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: '删除' }))

    const alert = await within(dialog).findByRole('alert')
    expect(alert).toHaveTextContent('已有任务在进行')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    // 整体回滚：课时仍在，选择模式与勾选保留（对话框 modal，外部元素 aria-hidden，用文本断言）
    expect(screen.getByText('已选 1 项')).toBeInTheDocument()
  })
})
