import { expect, test } from '@playwright/test'

test.use({ colorScheme: 'light' })

test('生产 CSP 下主题在首次切换、刷新和页面跳转后与按钮保持一致', async ({ page }) => {
  const response = await page.goto('/dashboard')
  expect(response?.headers()['content-security-policy']).toContain("script-src 'self'")
  await expect(page.locator('html')).not.toHaveClass(/dark/)

  await page.getByRole('button', { name: '切换到深色模式', exact: true }).click()
  await expect(page.locator('html')).toHaveClass(/dark/)
  await expect(page.getByRole('button', { name: '切换到浅色模式' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )

  await page.reload()
  await expect(page.locator('html')).toHaveClass(/dark/)
  await expect(page.getByRole('button', { name: '切换到浅色模式' })).toBeVisible()

  await page.getByRole('navigation', { name: '主导航' }).getByRole('link', { name: '课程' }).click()
  await expect(page).toHaveURL(/\/courses$/)
  await expect(page.locator('html')).toHaveClass(/dark/)

  // 整页跳转也需要在 React 挂载前恢复偏好。
  await page.goto('/progress')
  await expect(page.locator('html')).toHaveClass(/dark/)
  await page.getByRole('button', { name: '切换到浅色模式', exact: true }).click()
  await expect(page.locator('html')).not.toHaveClass(/dark/)
  await page.reload()
  await expect(page.getByRole('button', { name: '切换到深色模式' })).toBeVisible()
  await expect(page.locator('html')).not.toHaveClass(/dark/)
})
