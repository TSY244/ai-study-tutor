import { expect, test } from '@playwright/test'

// F4 阅读与进度 E2E：安全 Markdown 阅读（目录/代码/公式）、历史版本、
// 文档空状态、进度趋势与弱点过滤。MSW 提供 curated fixtures，不依赖后端。
// 两个 project（桌面/390px 窄屏）都会运行。

// 与 src/mocks/fixtures.ts 中 curated ID 保持一致
const COURSE_STATS_ID = '11111111-1111-4111-8111-111111111111'
const LESSON_RANDOM_ID = 'a0000000-0000-4000-8000-000000000001'
const LESSON_CONFIDENCE_ID = 'a0000000-0000-4000-8000-000000000003'
const NOTES_URL = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/notes`

test.describe('长文阅读', () => {
  test('笔记阅读：内容、目录锚点、代码复制、公式与历史版本', async ({ page }) => {
    await page.goto(NOTES_URL)

    // Markdown 内容与版本徽标
    const article = page.getByRole('article', { name: '文档内容' })
    await expect(article.getByRole('heading', { level: 1, name: '随机变量与分布' })).toBeVisible()
    await expect(page.getByText('R3', { exact: true })).toBeVisible()

    // 目录锚点：窄屏先展开折叠面板，再点击目录链接
    const isNarrow = (page.viewportSize()?.width ?? 1280) < 1280
    if (isNarrow) {
      // 窄屏目录是原生 details/summary（a11y 树中为 generic，按元素选择）
      await page.locator('summary', { hasText: '目录' }).click()
    }
    const toc = page.getByRole('navigation', { name: '文档目录' })
    await toc.getByRole('link', { name: '期望与方差' }).click()
    // 浏览器地址栏对 CJK 锚点做 URL 编码，解码后再断言
    await expect
      .poll(() => decodeURIComponent(page.url()), { timeout: 5000 })
      .toContain('#期望与方差')

    // 代码块复制按钮与 KaTeX 公式
    await expect(article.getByRole('button', { name: '复制' }).first()).toBeVisible()
    await expect(article.locator('.katex').first()).toBeVisible()

    // 历史版本：查看 R1 快照后回到当前版本
    await page.getByRole('button', { name: /历史版本/ }).click()
    const dialog = page.getByRole('dialog')
    await expect(dialog.getByText('当前版本')).toBeVisible()
    await dialog.locator('li', { hasText: 'R1' }).getByRole('button', { name: '查看' }).click()
    await expect(page.getByText(/正在查看历史版本 R1/)).toBeVisible()
    await page.getByRole('button', { name: '回到当前版本' }).click()
    await expect(page.getByText(/正在查看历史版本/)).toBeHidden()

    // 窄屏不得横向溢出（长表格/代码/公式在容器内滚动）
    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)
  })

  test('文档未生成时展示空状态', async ({ page }) => {
    await page.goto(`/courses/${COURSE_STATS_ID}/lessons/${LESSON_CONFIDENCE_ID}/notes`)
    await expect(page.getByText('笔记尚未生成')).toBeVisible()
    await expect(page.getByText(/导入素材并等待生成任务完成后/)).toBeVisible()
  })
})

test.describe('进度与弱点', () => {
  test('进度页展示课程趋势，弱点页按掌握状态过滤', async ({ page }) => {
    await page.goto('/progress')
    await expect(
      page.getByRole('img', { name: '统计学基础掌握度从 45% 变化到 62%，共 2 次记录' }),
    ).toBeVisible()
    await expect(page.getByText('完成更多测验后，这里会显示掌握度趋势。')).toBeVisible()

    await page.goto('/weaknesses')
    await expect(page.getByText('条件概率', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: '已掌握' }).click()
    await expect(page.getByText('期望的线性性', { exact: true })).toBeVisible()
    await expect(page.getByText('条件概率', { exact: true })).toBeHidden()
  })
})
