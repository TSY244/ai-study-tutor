import { expect, test } from '@playwright/test'

// F5 考试流程 E2E：开始作答 → 自动保存 → 刷新恢复 → 提交 →
// 200（全客观）/202（含主观题）两分支与批阅结果。MSW 提供含版本语义的
// 服务端行为，不依赖后端。两个 project（桌面/390px 窄屏）都会运行。
//
// 注意：浏览器内 MSW 的运行时状态（新建的 Attempt）随 page.reload 重置，
// 因此「刷新恢复」用 curated fixtures 中常驻的 demoAttempt 验证草稿恢复，
// 完整提交流程不在中途 reload。

const COURSE_STATS_ID = '11111111-1111-4111-8111-111111111111'
const LESSON_RANDOM_ID = 'a0000000-0000-4000-8000-000000000001'
const ATTEMPT_DEMO_ID = 'd0000000-0000-4000-8000-000000000001'
const QUIZZES_URL = `/courses/${COURSE_STATS_ID}/lessons/${LESSON_RANDOM_ID}/quizzes`

test.describe('考试流程', () => {
  test('202 分支：作答、自动保存、提交、批阅到出分', async ({ page }) => {
    await page.goto(QUIZZES_URL)
    // 第一份测验（含主观题）→ 202 分支
    await page
      .getByRole('listitem')
      .filter({ hasText: '随机变量与分布 · 随堂测验' })
      .getByRole('button', { name: '开始作答' })
      .click()

    // 专注模式：无主导航
    await expect(page.getByRole('heading', { name: '随机变量与分布 · 随堂测验' })).toBeVisible()
    await expect(page.getByRole('navigation', { name: '主导航' })).toBeHidden()

    // 答题：单选 + 简答
    await page.getByRole('radio', { name: 'B. 掷一枚骰子的点数' }).click()
    await page.getByLabel('作答内容').fill('E(X) = np，Var(X) = np(1-p)。')

    // 自动保存完成
    await expect(page.getByText('所有答案已同步')).toBeVisible({ timeout: 10000 })

    // 提交
    await page.getByRole('button', { name: '提交作答' }).click()
    await page.getByRole('button', { name: '确认提交' }).click()

    // 202：批阅进行中 → SSE 终态事件推送出分（精准失效 → REST 重取）
    await expect(page.getByText('AI 批阅进行中…')).toBeVisible({ timeout: 10000 })
    await expect(page.getByText(/客观题暂定分 5 \/ 15/)).toBeVisible()

    // Worker 完成批阅：任务存储定位批阅任务，推送 job.succeeded
    await page.evaluate(() => {
      const hooks = window as unknown as {
        __peekJobs?: () => Array<{ id: string; kind: string; status: string }>
        __jobStreamMock?: { push(frame: { id: number; event: string; data: unknown }): void }
      }
      const job = hooks
        .__peekJobs?.()
        .find(
          (item) =>
            item.kind === 'attempt_grade' &&
            (item.status === 'queued' || item.status === 'running'),
        )
      if (!job) throw new Error('找不到活动批阅任务')
      hooks.__jobStreamMock?.push({
        id: 990,
        event: 'job.succeeded',
        data: {
          job_id: job.id,
          status: 'succeeded',
          stage: '完成',
          progress: 100,
          message: '批阅完成',
          occurred_at: new Date().toISOString(),
        },
      })
    })

    await expect(page.getByText('已批阅').first()).toBeVisible({ timeout: 10000 })
    await expect(page.getByText('15 / 15', { exact: true })).toBeVisible()
    // 逐题反馈（提交后才下发）
    await expect(page.getByText('要点完整，表述清晰。')).toBeVisible()

    // 批阅任务在任务中心可见（全局可见性）
    await page.goto('/jobs')
    await expect(page.getByText('作答批阅').first()).toBeVisible()

    // 窄屏无横向溢出
    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)
  })

  test('刷新恢复：答案从 sessionStorage 草稿恢复并继续同步', async ({ page }) => {
    await page.goto(`/attempts/${ATTEMPT_DEMO_ID}`)
    await page.getByRole('radio', { name: 'B. 掷一枚骰子的点数' }).click()
    await page.getByLabel('作答内容').fill('刷新前写下的答案。')
    await expect(page.getByText('所有答案已同步')).toBeVisible({ timeout: 10000 })

    // 刷新：MSW 重置（服务端答案丢失），草稿应回填并以服务端基线重发
    await page.reload()
    await expect(page.getByLabel('作答内容')).toHaveValue('刷新前写下的答案。')
    await expect(page.getByRole('radio', { name: 'B. 掷一枚骰子的点数' })).toBeChecked()
    // 恢复后自动重发成功
    await expect(page.getByText('所有答案已同步')).toBeVisible({ timeout: 10000 })
  })

  test('200 分支：全客观题提交后直接出成绩', async ({ page }) => {
    await page.goto(QUIZZES_URL)
    await page
      .getByRole('listitem')
      .filter({ hasText: '期望与方差 · 客观题自测' })
      .getByRole('button', { name: '开始作答' })
      .click()

    await expect(page.getByRole('heading', { name: '期望与方差 · 客观题自测' })).toBeVisible()

    // 单选 + 多选 + 判断
    await page.getByRole('radio', { name: /B\. E\(X\) 是/ }).click()
    await page.getByRole('checkbox', { name: 'A. 二项分布' }).click()
    await page.getByRole('checkbox', { name: 'B. 泊松分布' }).click()
    await page.getByRole('radio', { name: '正确' }).click()

    await page.getByRole('button', { name: '提交作答' }).click()
    await page.getByRole('button', { name: '确认提交' }).click()

    // 200：直接 graded，无批阅等待
    await expect(page.getByText('已批阅').first()).toBeVisible({ timeout: 10000 })
    await expect(page.getByText('15 / 15', { exact: true })).toBeVisible()
    await expect(page.getByText(/批阅进行中/)).toBeHidden()
    // 作答锁定：无输入控件
    await expect(page.getByRole('radio')).toHaveCount(0)
    await expect(page.getByRole('checkbox')).toHaveCount(0)
  })
})
