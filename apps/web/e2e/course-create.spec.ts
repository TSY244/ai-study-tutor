import { expect, test } from '@playwright/test'

// F2 表单 E2E：真实浏览器验证课程创建流程（MSW curated handlers 模拟
// 服务端幂等语义，无后端）。两个 project（桌面/390px 窄屏）都会运行。
test.describe('新建课程', () => {
  test('创建成功后进入课程详情，课程列表（失效重取）包含新课程', async ({ page }) => {
    await page.goto('/courses/new')
    await expect(page.getByRole('heading', { name: '新建课程' })).toBeVisible()

    await page.getByLabel(/课程标题/).fill('E2E 新课程')
    await page.getByLabel(/课程描述/).fill('端到端验证创建流程。')
    await page.getByRole('button', { name: '创建课程' }).click()

    // 创建成功 -> 进入课程详情
    await expect(page).toHaveURL(/\/courses\/[0-9a-f-]{36}$/)
    await expect(page.getByRole('heading', { name: 'E2E 新课程' })).toBeVisible()

    // 回到列表：失效后重取，新课程在列
    await page
      .getByRole('navigation', { name: '主导航' })
      .getByRole('link', { name: '课程' })
      .click()
    await expect(page).toHaveURL(/\/courses$/)
    await expect(page.getByRole('link', { name: /E2E 新课程/ })).toBeVisible()

    // 窄屏（390px project）同样不得横向溢出
    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)
  })

  test('客户端校验：空标题不发请求，错误绑定字段且焦点归字段，修正后可创建', async ({ page }) => {
    await page.goto('/courses/new')

    const titleInput = page.getByLabel(/课程标题/)
    await page.getByRole('button', { name: '创建课程' }).click()

    // 错误绑定：字段提示 + aria-invalid；焦点归属出错字段；仍在原页
    await expect(page.getByText('请输入课程标题。')).toBeVisible()
    await expect(titleInput).toHaveAttribute('aria-invalid', 'true')
    await expect(titleInput).toBeFocused()
    await expect(page).toHaveURL(/\/courses\/new$/)

    // 修正后错误消除，可以完成创建
    await titleInput.fill('E2E 校验修正')
    await expect(page.getByText('请输入课程标题。')).toBeHidden()
    await page.getByRole('button', { name: '创建课程' }).click()
    await expect(page).toHaveURL(/\/courses\/[0-9a-f-]{36}$/)
    await expect(page.getByRole('heading', { name: 'E2E 校验修正' })).toBeVisible()
  })
})
