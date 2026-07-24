import { expect, test } from '@playwright/test'

import { ATTEMPT_DEMO_ID, JOB_FAILED_ID } from '../src/mocks/fixtures'

// F1 壳层 E2E：真实浏览器验证导航、面包屑、命令面板、任务中心、
// 专注模式与 reduced motion。数据由 MSW curated fixtures 提供（无后端）。
test.describe('App Shell 导航', () => {
  test('主导航到达各顶层页面且当前页状态生效', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: '学习总览' })).toBeVisible()

    const nav = page.getByRole('navigation', { name: '主导航' })
    const targets = [
      { name: '课程', url: /\/courses$/, heading: '课程' },
      { name: '弱点', url: /\/weaknesses$/, heading: '弱点' },
      { name: '进度', url: /\/progress$/, heading: '进度' },
      { name: '设置', url: /\/settings$/, heading: '设置' },
    ] as const

    for (const target of targets) {
      await nav.getByRole('link', { name: target.name }).click()
      await expect(page).toHaveURL(target.url)
      await expect(page.getByRole('heading', { name: target.heading }).first()).toBeVisible()
      await expect(nav.getByRole('link', { name: target.name })).toHaveAttribute(
        'aria-current',
        'page',
      )
    }

    await nav.getByRole('link', { name: '学习总览' }).click()
    await expect(page).toHaveURL(/\/dashboard$/)
  })

  test('课程 -> 课时 -> 笔记：面包屑与子视图导航', async ({ page }) => {
    await page.goto('/courses')
    await page.getByRole('link', { name: /统计学基础/ }).click()
    await expect(page).toHaveURL(/\/courses\/[0-9a-f-]+$/)
    await expect(page.getByRole('heading', { name: '统计学基础' })).toBeVisible()

    await page.getByRole('link', { name: /随机变量与分布/ }).click()
    // 课时 index 重定向到笔记
    await expect(page).toHaveURL(/\/lessons\/[0-9a-f-]+\/notes$/)

    const breadcrumb = page.getByRole('navigation', { name: '面包屑' })
    await expect(breadcrumb.getByRole('link', { name: '课程' })).toBeVisible()
    await expect(breadcrumb.getByRole('link', { name: '统计学基础' })).toBeVisible()
    await expect(breadcrumb.getByRole('link', { name: '随机变量与分布' })).toBeVisible()
    await expect(breadcrumb.getByText('笔记')).toHaveAttribute('aria-current', 'page')

    const subNav = page.getByRole('navigation', { name: '课时子视图' })
    await expect(subNav.getByRole('link', { name: '笔记' })).toHaveAttribute('aria-current', 'page')
    await subNav.getByRole('link', { name: '测验' }).click()
    await expect(page).toHaveURL(/\/quizzes$/)
    await expect(subNav.getByRole('link', { name: '测验' })).toHaveAttribute('aria-current', 'page')

    // 窄屏（390px project）同样不得横向溢出
    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)
  })

  test('命令面板：键盘打开、过滤、Enter 导航，焦点回到触发器', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: '学习总览' })).toBeVisible()

    await page.keyboard.press('ControlOrMeta+k')
    const input = page.getByRole('combobox', { name: '搜索命令' })
    await expect(input).toBeVisible()
    await expect(input).toBeFocused()

    await input.fill('课程列表')
    const option = page.getByRole('option', { name: '课程列表' })
    await expect(option).toHaveAttribute('aria-selected', 'true')
    await input.press('Enter')

    await expect(page).toHaveURL(/\/courses$/)
    await expect(page.getByRole('heading', { name: '课程' })).toBeVisible()
    // 对话框关闭后焦点回到触发按钮
    await expect(page.getByRole('button', { name: '打开命令面板（⌘K 或 Ctrl+K）' })).toBeFocused()
  })

  test('窄屏键盘路径：skip link 与命令面板可直达任务中心', async ({ page }) => {
    test.skip((page.viewportSize()?.width ?? 1280) > 450, '仅在 narrow-chromium project 验收')

    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: '学习总览' })).toBeVisible()

    await page.keyboard.press('Tab')
    await expect(page.getByRole('link', { name: '跳到主要内容' })).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(page.locator('#main-content')).toBeFocused()

    await page.keyboard.press('ControlOrMeta+k')
    const input = page.getByRole('combobox', { name: '搜索命令' })
    await expect(input).toBeFocused()
    await input.fill('任务中心')
    await input.press('Enter')

    await expect(page).toHaveURL(/\/jobs$/)
    await expect(page.getByRole('heading', { name: '任务中心' })).toBeVisible()

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)
  })

  test('任务中心 popover -> /jobs -> 失败任务详情', async ({ page }) => {
    await page.goto('/dashboard')
    const trigger = page.getByRole('button', { name: /任务中心/ })
    await expect(trigger).toBeVisible()
    await trigger.click()

    await expect(page.getByText('最近任务')).toBeVisible()
    await page.getByRole('link', { name: '查看全部' }).click()
    await expect(page).toHaveURL(/\/jobs$/)
    // 焦点回到触发器（popover 关闭）
    await expect(trigger).toBeFocused()

    await expect(page.getByRole('heading', { name: '任务中心' })).toBeVisible()
    await expect(page.getByText('运行中').first()).toBeVisible()
    await page.getByRole('link', { name: /测验生成/ }).click()
    await expect(page).toHaveURL(new RegExp(`/jobs/${JOB_FAILED_ID}$`))

    await expect(page.getByRole('heading', { name: '测验生成' })).toBeVisible()
    await expect(page.getByRole('alert')).toContainText('Claude 服务暂时不可用')
    await expect(page.getByText('claude_unavailable')).toBeVisible()
  })

  test('作答专注模式：无全局导航与任务中心，保留退出链路', async ({ page }) => {
    await page.goto(`/attempts/${ATTEMPT_DEMO_ID}`)

    await expect(page.getByRole('heading', { name: '随机变量与分布 · 随堂测验' })).toBeVisible()
    await expect(page.getByRole('navigation', { name: '主导航' })).toHaveCount(0)
    await expect(page.getByRole('button', { name: /任务中心/ })).toHaveCount(0)

    const exit = page.getByRole('link', { name: /退出作答/ })
    await expect(exit).toBeVisible()
    await exit.click()
    // 退出到所属课时的作答页
    await expect(page).toHaveURL(/\/lessons\/[0-9a-f-]+\/attempts$/)
  })
})

test.describe('reduced motion', () => {
  // Playwright 1.61 起 reducedMotion 不再是一等 test option，须经 contextOptions 传入
  test.use({ contextOptions: { reducedMotion: 'reduce' } })

  test('平滑滚动与过渡动效被关闭', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: '学习总览' })).toBeVisible()

    const scrollBehavior = await page.evaluate(
      () => getComputedStyle(document.documentElement).scrollBehavior,
    )
    expect(scrollBehavior).toBe('auto')

    const durationMs = await page.getByRole('link', { name: '跳到主要内容' }).evaluate((el) => {
      const raw = getComputedStyle(el).transitionDuration
      // 序列化可能是 "0.01ms" 或科学计数法秒 "1e-05s"
      return raw.endsWith('ms') ? parseFloat(raw) : parseFloat(raw) * 1000
    })
    expect(durationMs).toBeLessThanOrEqual(0.01)
  })
})
