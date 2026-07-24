import { screen, within } from '@testing-library/react'
import { http } from 'msw'
import { describe, expect, it } from 'vitest'

import { server } from '@/mocks/server'
import { problemResponse } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

describe('学习总览', () => {
  it('展示课程、进行中任务与系统状态（链路：路由 -> Query -> 生成 Client -> MSW）', async () => {
    renderWithRouter('/dashboard')

    expect(await screen.findByRole('heading', { name: '学习总览' })).toBeInTheDocument()
    // 我的课程（curated fixture）
    expect(await screen.findByRole('link', { name: /统计学基础/ })).toBeInTheDocument()
    // 进行中的任务（fixture：lesson_import running 45%）
    expect(await screen.findByText('生成学生笔记')).toBeInTheDocument()
    // 系统状态
    expect(await screen.findByText('版本 0.3.0-draft')).toBeInTheDocument()
    expect(screen.getByText('Claude 连接')).toBeInTheDocument()
    // 主导航当前页状态
    const nav = screen.getByRole('navigation', { name: '主导航' })
    expect(within(nav).getByRole('link', { name: '学习总览' })).toHaveAttribute(
      'aria-current',
      'page',
    )
  })

  it('Problem 错误展示错误码、请求 ID 与重试入口，不清空页面结构', async () => {
    server.use(
      http.get('*/api/v1/system/status', () =>
        problemResponse({
          status: 503,
          code: 'maintenance_mode',
          title: '维护中',
          detail: '系统正在执行一致性备份，暂时拒绝写入。',
        }),
      ),
    )
    renderWithRouter('/dashboard')

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('maintenance_mode')).toBeInTheDocument()
    expect(screen.getByText('req-test-1')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument()
    // 页面骨架仍在，不被错误清空
    expect(screen.getByRole('heading', { name: '学习总览' })).toBeInTheDocument()
  })
})
