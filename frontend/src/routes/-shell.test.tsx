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

  it('设置：创建 Claude 网关级连通性测试任务并跳转到任务详情', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', '/api/v1/system/claude-tests')
    const { router } = renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '测试 Claude 网关连接' }))

    await waitFor(() => {
      expect(router.state.location.pathname).toMatch(/^\/jobs\/[0-9a-f-]{36}$/)
    })
    expect(requests).toHaveLength(1)
    expect(isIdempotencyKey(requests[0]?.key ?? '')).toBe(true)
    stop()
  })

  it('设置：安全展示 Claude Key，并通过密码框新增但不回显完整 Key', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', '/api/v1/system/claude-keys')
    renderWithRouter('/settings')

    expect(await screen.findByText('主线路')).toBeInTheDocument()
    expect(screen.getByText('sk-ant-...7K2P')).toBeInTheDocument()
    expect(screen.getByText('已启用 1')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '添加 Key' }))
    const secretInput = screen.getByLabelText(/^Anthropic API Key/)
    expect(secretInput).toHaveAttribute('type', 'password')
    const transientSecret = ['runtime', 'browser', 'credential'].join('-')
    await user.type(screen.getByLabelText(/^名称/), '浏览器新增')
    await user.type(secretInput, transientSecret)
    await user.type(screen.getByLabelText(/^模型/), 'claude-test')
    await user.click(screen.getByRole('button', { name: '显示 API Key' }))
    expect(secretInput).toHaveAttribute('type', 'text')
    await user.click(screen.getByRole('button', { name: '创建 Key' }))

    expect(await screen.findByText('浏览器新增')).toBeInTheDocument()
    await waitFor(() => expect(requests[0]?.body).not.toBeNull())
    expect((requests[0]?.body as Record<string, unknown>).api_key).toBe(transientSecret)
    expect(isIdempotencyKey(requests[0]?.key ?? '')).toBe(true)
    expect(screen.queryByText(transientSecret)).not.toBeInTheDocument()
    stop()
  })

  it('设置：编辑 Key 留空密钥时 PATCH 省略 api_key，并确认停用', async () => {
    const user = userEvent.setup()
    const patchCapture = captureRequests('PATCH', '/api/v1/system/claude-keys/ck_demo_primary')
    renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '编辑主线路' }))
    const secretInput = screen.getByLabelText('Anthropic API Key（可选）')
    expect(secretInput).toHaveValue('')
    const nameInput = screen.getByLabelText(/^名称/)
    await user.clear(nameInput)
    await user.type(nameInput, '主线路·已编辑')
    await user.click(screen.getByRole('button', { name: '保存修改' }))

    expect(await screen.findByText('主线路·已编辑')).toBeInTheDocument()
    await waitFor(() => expect(patchCapture.requests[0]?.body).not.toBeNull())
    expect(patchCapture.requests[0]?.body).not.toHaveProperty('api_key')

    await user.click(screen.getByRole('button', { name: '停用主线路·已编辑' }))
    const dialog = screen.getByRole('dialog', { name: '停用 Claude Key' })
    await user.click(within(dialog).getByRole('button', { name: '停用' }))
    await waitFor(() => expect(patchCapture.requests).toHaveLength(2))
    expect(patchCapture.requests[1]?.body).toEqual({ enabled: false })
    expect(await screen.findByText(/已停用 2/)).toBeInTheDocument()
    patchCapture.stop()
  })

  it('设置：创建 Key 留空 Base URL 时 POST 省略 base_url', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', '/api/v1/system/claude-keys')
    renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '添加 Key' }))
    await user.type(screen.getByLabelText(/^名称/), '默认网关线路')
    await user.type(screen.getByLabelText(/^模型/), 'claude-test')
    await user.type(screen.getByLabelText(/^Anthropic API Key/), 'runtime-blank-base-url')
    // Base URL 留空：创建时不发送该字段（回退部署环境变量）。
    await user.click(screen.getByRole('button', { name: '创建 Key' }))

    expect(await screen.findByText('默认网关线路')).toBeInTheDocument()
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]?.body).not.toHaveProperty('base_url')
    stop()
  })

  it('设置：编辑 Key 清空 Base URL 时 PATCH 发送 base_url 空串', async () => {
    const user = userEvent.setup()
    const patchCapture = captureRequests('PATCH', '/api/v1/system/claude-keys/ck_demo_primary')
    renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '编辑主线路' }))
    const baseUrlInput = screen.getByLabelText('Base URL')
    expect(baseUrlInput).toHaveValue('https://gw-primary.example')
    await user.clear(baseUrlInput)
    await user.click(screen.getByRole('button', { name: '保存修改' }))

    // 编辑时空串表示清除（回退环境变量），必须显式发送。
    await waitFor(() => expect(patchCapture.requests).toHaveLength(1))
    await waitFor(() => expect(patchCapture.requests[0]?.body).not.toBeNull())
    expect(patchCapture.requests[0]?.body).toHaveProperty('base_url', '')
    patchCapture.stop()
  })

  it('设置：Base URL 非 https 时前端拦截且不提交', async () => {
    const user = userEvent.setup()
    const { requests, stop } = captureRequests('POST', '/api/v1/system/claude-keys')
    renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '添加 Key' }))
    await user.type(screen.getByLabelText(/^名称/), '非法网关')
    await user.type(screen.getByLabelText(/^模型/), 'claude-test')
    await user.type(screen.getByLabelText(/^Anthropic API Key/), 'runtime-invalid-base-url')
    await user.type(screen.getByLabelText('Base URL'), 'http://insecure.example')
    await user.click(screen.getByRole('button', { name: '创建 Key' }))

    expect(await screen.findByText('Base URL 必须是 https:// 开头的地址')).toBeInTheDocument()
    expect(requests).toHaveLength(0)
    stop()
  })

  it('设置：应用指定 Key 到运行服务前要求确认，且浏览器只发送 Key ID', async () => {
    const user = userEvent.setup()
    const saveCapture = captureRequests(
      'POST',
      '/api/v1/system/claude-keys/ck_demo_primary/claude-code-settings',
    )
    renderWithRouter('/settings')

    const keyCard = await screen.findByRole('article', { name: 'Claude Key 主线路' })
    const applyButton = within(keyCard).getByRole('button', {
      name: '应用主线路到运行服务的 Claude Code',
    })
    expect(applyButton).toHaveTextContent('应用到运行服务的 Claude Code')

    await user.click(applyButton)
    const dialog = screen.getByRole('dialog', { name: '应用到运行服务的 Claude Code' })
    expect(dialog).toHaveTextContent('运行环境 ~/.claude/settings.json')
    expect(dialog).toHaveTextContent('Worker Claude Code 任务')
    expect(dialog).not.toHaveTextContent('本机')
    expect(saveCapture.requests).toHaveLength(0)

    await user.click(within(dialog).getByRole('button', { name: '确认应用' }))
    await waitFor(() => expect(saveCapture.requests).toHaveLength(1))
    expect(saveCapture.requests[0]?.body).toBeNull()
    expect(await screen.findByRole('status')).toHaveTextContent(
      '已将「主线路」应用到运行服务的 Claude Code。',
    )
    expect(screen.queryByText('sk-ant-demo-primary-DO-NOT-LEAK')).not.toBeInTheDocument()
    saveCapture.stop()
  })

  it('设置：单 Key 测试跳转任务详情，删除操作需要确认', async () => {
    const user = userEvent.setup()
    const testCapture = captureRequests('POST', '/api/v1/system/claude-keys/ck_demo_primary/test')
    const deleteCapture = captureRequests('DELETE', '/api/v1/system/claude-keys/ck_demo_backup')
    const { router } = renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '测试主线路' }))
    await waitFor(() => {
      expect(router.state.location.pathname).toMatch(/^\/jobs\/[0-9a-f-]{36}$/)
    })
    expect(testCapture.requests).toHaveLength(1)
    expect(isIdempotencyKey(testCapture.requests[0]?.key ?? '')).toBe(true)

    await router.navigate({ to: '/settings' })
    await user.click(await screen.findByRole('button', { name: '删除备用线路' }))
    const dialog = screen.getByRole('dialog', { name: '删除 Claude Key' })
    expect(deleteCapture.requests).toHaveLength(0)
    await user.click(within(dialog).getByRole('button', { name: '删除' }))
    await waitFor(() => expect(deleteCapture.requests).toHaveLength(1))
    expect(screen.queryByText('备用线路')).not.toBeInTheDocument()
    testCapture.stop()
    deleteCapture.stop()
  })
})
