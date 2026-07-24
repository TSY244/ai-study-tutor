import { expect, test } from '@playwright/test'

// F3 主流程 E2E：创建课程 -> 新建课时 -> 导入文本素材 -> 后台生成任务全局可见
// （课时页任务卡 / 任务详情 / 任务中心角标 / /jobs 列表）。
// MSW 模拟服务端幂等与任务状态，不依赖后端；两个 project（桌面/390px）都会运行。
test.describe('课时导入与全局任务', () => {
  test('创建课程到导入文本课时，离开页面后任务仍全局可见', async ({ page }) => {
    // 1. 创建课程
    await page.goto('/courses/new')
    await page.getByLabel(/课程标题/).fill('E2E 导入流程课')
    await page.getByRole('button', { name: '创建课程' }).click()
    await expect(page).toHaveURL(/\/courses\/[0-9a-f-]{36}$/)

    // 2. 新建课时（空课程页有节区按钮与空状态两个入口，取第一个）；课次默认 1
    await page.getByRole('link', { name: '新建课时' }).first().click()
    await expect(page).toHaveURL(/\/lessons\/new$/)
    await expect(page.getByLabel(/课次/)).toHaveValue('1')
    await page.getByLabel(/课时标题/).fill('第一课时')
    await page.getByRole('button', { name: '创建课时' }).click()

    // 3. 课时页（默认进入笔记子视图）导入文本素材
    await expect(page).toHaveURL(/\/lessons\/[0-9a-f-]{36}\/notes$/)
    await page.getByLabel(/文本内容/).fill('第一章：导数与极限。')
    await page.getByRole('button', { name: '导入文本' }).click()

    // 4. 后台生成任务卡出现（与上传进度分离），记录新任务地址
    await expect(page.getByRole('progressbar', { name: '生成任务进度' })).toBeVisible()
    const jobDetailHref = await page
      .getByRole('link', { name: '查看任务详情' })
      .getAttribute('href')
    expect(jobDetailHref).toMatch(/^\/jobs\/[0-9a-f-]{36}$/)

    // 5. 任务详情页：标题与进度可见
    await page.getByRole('link', { name: '查看任务详情' }).click()
    await expect(page).toHaveURL(/\/jobs\/[0-9a-f-]{36}$/)
    await expect(page.getByRole('heading', { name: '课时导入' })).toBeVisible()

    // 6. 任务中心在全局壳中仍可见；从任务中心进入 /jobs，新建任务在列表中
    const jobsTrigger = page.getByRole('button', { name: /任务中心/ })
    await expect(jobsTrigger).toHaveAccessibleName(/任务进行中/)
    await jobsTrigger.click()
    await page.getByRole('link', { name: '查看全部' }).click()
    await expect(page).toHaveURL(/\/jobs$/)
    // demo 数据中也有其他"课时导入"任务：按 href 精确匹配新建任务
    await expect(page.locator(`a[href="${jobDetailHref}"]`)).toBeVisible()

    // 窄屏（390px project）同样不得横向溢出
    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)
  })
})
