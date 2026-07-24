import { expect, test } from '@playwright/test'

// F0 smoke：验证工程底座端到端可运行——Vite dev server + MSW 接线 +
// 路由（/ -> /dashboard 重定向）+ Query + 生成 Client。后端不参与。
test.describe('应用底座 smoke', () => {
  test('根路径进入学习总览并展示 MSW 系统状态', async ({ page }) => {
    await page.goto('/')

    await expect(page).toHaveURL(/\/dashboard$/)
    await expect(page.getByRole('heading', { name: '学习总览' })).toBeVisible()
    // MSW curated fixture：系统状态正常、版本来自设计草案契约
    await expect(page.getByText('版本 0.3.0-draft')).toBeVisible()
    await expect(page.getByText('Claude 连接')).toBeVisible()
  })

  test('键盘可达：skip link 跳转主内容，导航有当前页状态', async ({ page }) => {
    await page.goto('/dashboard')
    // 等待 React 挂载完成再开始键盘操作
    await expect(page.getByRole('heading', { name: '学习总览' })).toBeVisible()

    const skipLink = page.getByRole('link', { name: '跳到主要内容' })
    await page.keyboard.press('Tab')
    await expect(skipLink).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(page.locator('#main-content')).toBeFocused()

    const nav = page.getByRole('navigation', { name: '主导航' })
    await expect(nav.getByRole('link', { name: '学习总览' })).toHaveAttribute(
      'aria-current',
      'page',
    )
  })

  test('无横向溢出，文本不被截断遮挡', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: '学习总览' })).toBeVisible()

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)
  })
})
