import { expect, test, type Page } from '@playwright/test'

// F6 任务事件流 E2E：fetch 传输的 SSE 客户端（同源 /api/v1/jobs/stream）
// + window 测试钩子（__jobStreamMock 推送/断开，__peekJobs 定位任务）。
// MSW 流事件 → REST 状态桥模拟 Worker 写库，保证推送后 REST 重取一致。
// 覆盖：实时推送、终态跨页一致、resync 对账、心跳、断线退避重连。
// 两个 project（桌面/390px 窄屏）都会运行。

const JOB_RUNNING_ID = 'b0000000-0000-4000-8000-000000000001'

interface StreamFrame {
  id?: number
  event: string
  data: unknown
}

async function pushFrame(page: Page, frame: StreamFrame): Promise<void> {
  await page.evaluate(async (f) => {
    const mock = (
      window as unknown as {
        __jobStreamMock?: { push(frame: StreamFrame): void }
      }
    ).__jobStreamMock
    if (!mock) throw new Error('__jobStreamMock 未挂载（MSW 未启动？）')
    mock.push(f)
  }, frame)
}

function pushJobProgress(page: Page, id: number, progress: number): Promise<void> {
  return pushFrame(page, {
    id,
    event: 'job.progress',
    data: {
      job_id: JOB_RUNNING_ID,
      status: 'running',
      stage: '生成教师大纲',
      progress,
      message: null,
      occurred_at: new Date().toISOString(),
    },
  })
}

function pushJobSucceeded(page: Page, id: number): Promise<void> {
  return pushFrame(page, {
    id,
    event: 'job.succeeded',
    data: {
      job_id: JOB_RUNNING_ID,
      status: 'succeeded',
      stage: '完成',
      progress: 100,
      message: '课时已生成',
      occurred_at: new Date().toISOString(),
    },
  })
}

test.describe('任务事件流', () => {
  test('实时推送：进度就地更新，终态后任务列表/课程页/任务中心跨页一致', async ({ page }) => {
    await page.goto('/jobs')
    const bar = page.getByRole('progressbar', { name: '课时导入进度' })
    await expect(bar).toHaveAttribute('aria-valuenow', '45')
    await expect(page.getByText('生成学生笔记').first()).toBeVisible()

    // 进度事件：就地合并，无整页刷新
    await pushJobProgress(page, 900, 80)
    await expect(bar).toHaveAttribute('aria-valuenow', '80')
    await expect(page.getByText('生成教师大纲').first()).toBeVisible()

    // 终态事件：MSW 桥翻转任务与课时 → 精准失效 → 列表显示已成功
    await pushJobSucceeded(page, 901)
    await expect(page.getByText('已成功').first()).toBeVisible({ timeout: 10000 })
    await expect(page.getByRole('progressbar', { name: '课时导入进度' })).toHaveCount(0)

    // 课程页（应用内导航，保持 MSW 状态）：贝叶斯定理从「生成中」流转为「就绪」
    await page
      .getByRole('navigation', { name: '主导航' })
      .getByRole('link', { name: '课程' })
      .click()
    await page
      .getByRole('link', { name: /统计学基础/ })
      .first()
      .click()
    const lessonRow = page.getByRole('listitem').filter({ hasText: '贝叶斯定理' })
    await expect(lessonRow.getByText('就绪')).toBeVisible({ timeout: 10000 })

    // 任务中心角标归零
    await expect(page.getByRole('button', { name: '任务中心，暂无进行中的任务' })).toBeVisible()

    // 窄屏无横向溢出
    const hasHorizontalOverflow = await page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
    )
    expect(hasHorizontalOverflow).toBe(false)
  })

  test('stream.resync_required 与心跳：游标更新、REST 对账、诊断面板可见', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.getByText('任务事件流（开发诊断）')).toBeVisible()
    await expect(page.getByText('已连接')).toBeVisible()

    // resync：游标更新为高水位，最近断档对账时间出现（REST 对账由客户端发起）
    await pushFrame(page, {
      id: 1042,
      event: 'stream.resync_required',
      data: {
        reason: 'history_expired',
        high_watermark_event_id: 1042,
        active_jobs_url: '/api/v1/jobs?active=true',
      },
    })
    await expect(page.getByText('1042')).toBeVisible({ timeout: 10000 })
    await expect(page.getByText(/stream\.resync_required ×1/)).toBeVisible()

    // 心跳：只更新活性，不推进游标（游标仍是 1042）
    await pushFrame(page, {
      event: 'heartbeat',
      data: { server_time: '2026-07-19T08:30:00Z' },
    })
    await expect(page.getByText('2026-07-19T08:30:00Z')).toBeVisible({ timeout: 10000 })
    await expect(page.getByText('1042')).toBeVisible()
  })

  test('断线后按退避自动重连，新连接携带 Last-Event-ID 游标', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('button', { name: /任务中心/ })).toBeVisible()

    // 先推进游标，再断线
    await pushJobProgress(page, 950, 70)
    const before = await page.evaluate(() => {
      const mock = (
        window as unknown as { __jobStreamMock?: { connections: number; closeAll(): void } }
      ).__jobStreamMock
      if (!mock) throw new Error('__jobStreamMock 未挂载')
      const count = mock.connections
      mock.closeAll()
      return count
    })

    // 退避（1s ±25%）后自动重连：连接数增加，且携带最新游标
    await expect
      .poll(
        async () =>
          page.evaluate(() => {
            const mock = (
              window as unknown as {
                __jobStreamMock?: { connections: number; lastCursor: string | null }
              }
            ).__jobStreamMock
            return { connections: mock?.connections ?? 0, cursor: mock?.lastCursor ?? null }
          }),
        { timeout: 15000 },
      )
      .toEqual({ connections: before + 1, cursor: '950' })
  })
})
