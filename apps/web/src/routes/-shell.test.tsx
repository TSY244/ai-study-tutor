import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { isIdempotencyKey } from '@/lib/idempotency'
import { COURSE_STATS_ID, LESSON_RANDOM_ID } from '@/mocks/fixtures'
import { captureRequests } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

const LESSON_NOTES_PATH = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/notes`

describe('App Shell', () => {
  it('课时页面面包屑：课程 / 课程名 / 课时名 / 子视图，末级为当前页', async () => {
    renderWithRouter(LESSON_NOTES_PATH)

    const breadcrumb = await screen.findByRole('navigation', { name: '面包屑' })
    const items = await within(breadcrumb).findAllByRole('listitem')
    expect(items).toHaveLength(4)
    expect(within(breadcrumb).getByRole('link', { name: '课程' })).toBeInTheDocument()
    expect(await within(breadcrumb).findByRole('link', { name: '统计学基础' })).toBeInTheDocument()
    expect(
      await within(breadcrumb).findByRole('link', { name: '随机变量与分布' }),
    ).toBeInTheDocument()
    // 末级不是链接，是 aria-current 文本
    const current = within(breadcrumb).getByText('笔记')
    expect(current).toHaveAttribute('aria-current', 'page')
    expect(current.closest('a')).toBeNull()
  })

  it('课时子视图导航带 aria-current，可切换到其他子视图', async () => {
    const user = userEvent.setup()
    const { router } = renderWithRouter(LESSON_NOTES_PATH)

    const subNav = await screen.findByRole('navigation', { name: '课时子视图' })
    expect(within(subNav).getByRole('link', { name: '笔记' })).toHaveAttribute(
      'aria-current',
      'page',
    )

    await user.click(within(subNav).getByRole('link', { name: '大纲' }))
    await waitFor(() => {
      expect(router.state.location.pathname).toBe(
        `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/outline`,
      )
    })
    expect(within(subNav).getByRole('link', { name: '大纲' })).toHaveAttribute(
      'aria-current',
      'page',
    )
  })

  it('任务中心：角标显示进行中数量，面板列出最近任务并可进入 /jobs', async () => {
    const user = userEvent.setup()
    const { router } = renderWithRouter('/dashboard')

    // fixture：1 个进行中任务（running）
    const trigger = await screen.findByRole('button', {
      name: '任务中心，1 个任务进行中',
    })
    await user.click(trigger)

    expect(await screen.findByText('最近任务')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '查看全部' })).toBeInTheDocument()

    await user.click(screen.getByRole('link', { name: '查看全部' }))
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/jobs')
    })
    // popover 关闭，焦点回到触发器
    expect(trigger).toHaveFocus()
  })

  it('命令面板：过滤 + 键盘选择导航，关闭后焦点回到触发按钮', async () => {
    const user = userEvent.setup()
    const { router } = renderWithRouter('/dashboard')

    const trigger = await screen.findByRole('button', { name: '打开命令面板（⌘K 或 Ctrl+K）' })
    await user.click(trigger)

    const input = await screen.findByRole('combobox', { name: '搜索命令' })
    expect(input).toHaveFocus()

    await user.type(input, '进度')
    const option = await screen.findByRole('option', { name: '学习进度' })
    expect(option).toHaveAttribute('aria-selected', 'true')
    expect(input).toHaveAttribute('aria-activedescendant', option.id)

    await user.keyboard('{Enter}')
    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/progress')
    })
    expect(screen.queryByRole('combobox', { name: '搜索命令' })).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('⌘K / Ctrl+K 全局快捷键打开命令面板', async () => {
    const user = userEvent.setup()
    renderWithRouter('/dashboard')

    await user.keyboard('{Meta>}k{/Meta}')
    expect(await screen.findByRole('combobox', { name: '搜索命令' })).toBeInTheDocument()

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('combobox', { name: '搜索命令' })).not.toBeInTheDocument()

    await user.keyboard('{Control>}k{/Control}')
    expect(await screen.findByRole('combobox', { name: '搜索命令' })).toBeInTheDocument()
  })

  it('设置：创建 Claude 连通性测试任务并跳转到任务详情', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', '/api/v1/system/claude-tests')
    const { router } = renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '测试 Claude 连接' }))

    await waitFor(() => {
      expect(router.state.location.pathname).toMatch(/^\/jobs\/[0-9a-f-]{36}$/)
    })
    expect(requests).toHaveLength(1)
    expect(isIdempotencyKey(requests[0]?.key ?? '')).toBe(true)
    stop()
  })
})
