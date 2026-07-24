import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ATTEMPT_DEMO_ID } from '@/mocks/fixtures'
import { renderWithRouter } from '@/test/utils'

describe('作答专注模式', () => {
  it('不渲染 App Shell 全局导航，保留退出链路与作答状态', async () => {
    renderWithRouter(`/attempts/${ATTEMPT_DEMO_ID}`)

    // 等专注模式内容就绪后再断言 Shell 不存在
    expect(
      await screen.findByRole('heading', { name: '随机变量与分布 · 随堂测验' }),
    ).toBeInTheDocument()

    // 专注模式：无主导航、无任务中心
    expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /任务中心/ })).not.toBeInTheDocument()

    // 退出链路 + 状态标签
    expect(screen.getByRole('link', { name: /退出作答/ })).toBeInTheDocument()
    expect(await screen.findByText('作答中')).toBeInTheDocument()
  })
})
