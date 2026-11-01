import { expect, test } from '@playwright/test'

import { ATTEMPT_DEMO_ID } from '../src/mocks/fixtures'

test.use({ colorScheme: 'light' })

test('未设置偏好时跟随系统主题变化，手动选择后保留用户偏好', async ({ page }) => {
  await page.goto('/dashboard')
  await expect(page.getByRole('button', { name: '切换到深色模式' })).toBeVisible()
  await expect(page.locator('html')).not.toHaveClass(/dark/)

  await page.emulateMedia({ colorScheme: 'dark' })
  await expect(page.locator('html')).toHaveClass(/dark/)
  await expect(page.getByRole('button', { name: '切换到浅色模式' })).toBeVisible()
  expect(await page.evaluate(() => localStorage.getItem('theme'))).toBeNull()

  await page.getByRole('button', { name: '切换到浅色模式' }).click()
  await page.emulateMedia({ colorScheme: 'light' })
  await page.emulateMedia({ colorScheme: 'dark' })
  await expect(page.locator('html')).not.toHaveClass(/dark/)
  expect(await page.evaluate(() => localStorage.getItem('theme'))).toBe('light')
})

test('偏好写入失败时，系统主题变化仍保留本次手动选择', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await page.addInitScript(() => {
    const setItem = Storage.prototype.setItem
    Storage.prototype.setItem = function (key, value) {
      if (key === 'theme') throw new DOMException('Storage unavailable', 'QuotaExceededError')
      setItem.call(this, key, value)
    }
  })
  await page.goto('/dashboard')
  await page.getByRole('button', { name: '切换到浅色模式' }).click()
  await expect(page.locator('html')).not.toHaveClass(/dark/)
  expect(await page.evaluate(() => localStorage.getItem('theme'))).toBeNull()

  await page.emulateMedia({ colorScheme: 'light' })
  await page.emulateMedia({ colorScheme: 'dark' })
  await expect(page.locator('html')).not.toHaveClass(/dark/)
  await expect(page.getByRole('button', { name: '切换到深色模式' })).toBeVisible()
})

test('主题跨标签页同步，专注作答页保持同步且刷新后保留偏好', async ({ page, context }) => {
  await page.goto('/dashboard')
  const other = await context.newPage()
  await other.goto('/dashboard')
  await expect(other.getByRole('button', { name: '切换到深色模式' })).toBeVisible()

  await page.getByRole('button', { name: '切换到深色模式' }).click()
  await expect(other.locator('html')).toHaveClass(/dark/)
  await expect(other.getByRole('button', { name: '切换到浅色模式' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )

  await other.goto(`/attempts/${ATTEMPT_DEMO_ID}`)
  await expect(other.getByRole('heading', { name: '随机变量与分布 · 随堂测验' })).toBeVisible()
  await expect(other.getByRole('button', { name: '切换到浅色模式' })).toHaveCount(0)
  await expect(other.locator('html')).toHaveClass(/dark/)
  await page.getByRole('button', { name: '切换到浅色模式' }).click()
  await expect(other.locator('html')).not.toHaveClass(/dark/)
  await other.reload()
  await expect(other.getByRole('heading', { name: '随机变量与分布 · 随堂测验' })).toBeVisible()
  await expect(other.locator('html')).not.toHaveClass(/dark/)
})
