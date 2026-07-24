import { expect, test } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

async function expectNoWcagViolations(page: Parameters<typeof AxeBuilder>[0]['page']) {
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()

  expect(
    results.violations,
    results.violations
      .map((violation) => `${violation.id}: ${violation.help} (${violation.nodes.length})`)
      .join('\n'),
  ).toEqual([])
}

test.describe('production Docker release', () => {
  test('Caddy deep link, same-origin API/SSE, and seeded reading flow', async ({
    page,
  }, testInfo) => {
    const pageErrors: string[] = []
    page.on('pageerror', (error) => pageErrors.push(error.message))

    const streamResponse = page.waitForResponse(
      (response) =>
        response.url().endsWith('/api/v1/jobs/stream') && response.request().method() === 'GET',
    )
    const systemStatusResponse = page.waitForResponse(
      (response) =>
        response.url().endsWith('/api/v1/system/status') && response.request().method() === 'GET',
    )

    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: '学习总览' })).toBeVisible()
    await expect(page.getByRole('link', { name: /演示课程：计算机科学导论/ })).toBeVisible()

    const systemStatus = await systemStatusResponse
    expect(systemStatus.status()).toBe(200)
    await expect(page.getByRole('heading', { name: '系统状态' })).toBeVisible()
    await expect(page.getByText('无法获取系统状态')).toHaveCount(0)

    const sse = await streamResponse
    expect(sse.headers()['content-type']).toContain('text/event-stream')
    expect(new URL(sse.url()).origin).toBe(new URL(page.url()).origin)

    await page.getByRole('link', { name: /演示课程：计算机科学导论/ }).click()
    await expect(page.getByRole('heading', { name: '演示课程：计算机科学导论' })).toBeVisible()
    await page.getByRole('link', { name: /算法复杂度入门/ }).click()
    await page
      .getByRole('navigation', { name: '课时子视图' })
      .getByRole('link', { name: '原文' })
      .click()

    const article = page.getByRole('article', { name: '文档内容' })
    await expect(article.getByRole('heading', { level: 1, name: '算法复杂度入门' })).toBeVisible()
    await expect(article.getByText('O(log n)', { exact: false })).toBeVisible()

    await page.screenshot({ path: testInfo.outputPath('reading-flow.png'), fullPage: true })
    expect(pageErrors).toEqual([])
  })

  test('narrow keyboard path has focus and no page overflow', async ({ page }, testInfo) => {
    test.skip((page.viewportSize()?.width ?? 1280) > 450, 'narrow viewport only')

    await page.goto('/courses')
    await expect(page.getByRole('heading', { name: '课程' }).first()).toBeVisible()

    await page.keyboard.press('Tab')
    await expect(page.getByRole('link', { name: '跳到主要内容' })).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(page.locator('#main-content')).toBeFocused()

    await page.keyboard.press('ControlOrMeta+k')
    const commandSearch = page.getByRole('combobox', { name: '搜索命令' })
    await expect(commandSearch).toBeFocused()
    await commandSearch.fill('任务中心')
    await commandSearch.press('Enter')
    await expect(page).toHaveURL(/\/jobs$/)
    await expect(page.getByRole('heading', { name: '任务中心' })).toBeVisible()

    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)

    await page.screenshot({ path: testInfo.outputPath('narrow-jobs.png'), fullPage: true })
  })

  test('dashboard, courses, and jobs have no WCAG A/AA violations', async ({ page }) => {
    for (const route of ['/dashboard', '/courses', '/jobs']) {
      await page.goto(route)
      await expect(page.locator('#main-content')).toBeVisible()
      await expectNoWcagViolations(page)
    }
  })
})
