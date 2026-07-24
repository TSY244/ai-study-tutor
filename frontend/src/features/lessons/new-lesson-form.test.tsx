import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { isIdempotencyKey } from '@/lib/idempotency'
import { COURSE_STATS_ID } from '@/mocks/fixtures'
import { captureRequests } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

/** 新建课时：课次默认值、创建流程（幂等 Key + 精确失效 + 导航）。 */
describe('NewLessonForm', () => {
  it('课次默认最大值+1；创建成功带 Idempotency-Key 并进入课时页（导入面板可见）', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', `/api/v1/courses/${COURSE_STATS_ID}/lessons`)
    const { router } = renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/new`)

    // 演示课程已有 3 个课时（最大课次 3）→ 默认 4
    const ordinalInput = await screen.findByLabelText(/课次/)
    await waitFor(() => expect(ordinalInput).toHaveValue(4))

    await user.type(await screen.findByLabelText(/课时标题/), '  方差分析入门  ')
    await user.click(screen.getByRole('button', { name: '创建课时' }))

    await waitFor(() =>
      expect(router.state.location.pathname).toMatch(
        new RegExp(`^/courses/${COURSE_STATS_ID}/lessons/[0-9a-f-]{36}`),
      ),
    )
    // 课时页：draft 状态显示导入面板
    expect(await screen.findByRole('heading', { name: '导入素材' })).toBeInTheDocument()

    expect(requests).toHaveLength(1)
    expect(isIdempotencyKey(requests[0]?.key ?? '')).toBe(true)
    expect(requests[0]?.body).toEqual({ ordinal: 4, title: '方差分析入门' })
    stop()
  })

  it('客户端校验：空标题绑定错误且不发请求', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', `/api/v1/courses/${COURSE_STATS_ID}/lessons`)
    renderWithRouter(`/courses/${COURSE_STATS_ID}/lessons/new`)

    const titleInput = await screen.findByLabelText(/课时标题/)
    await user.click(screen.getByRole('button', { name: '创建课时' }))

    expect(await screen.findByText('请输入课时标题。')).toBeInTheDocument()
    expect(titleInput).toHaveAttribute('aria-invalid', 'true')
    expect(requests).toHaveLength(0)
    stop()
  })
})
