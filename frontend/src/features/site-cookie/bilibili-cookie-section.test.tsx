import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { BilibiliCookieStatus } from '@/api/generated/model'
import { server } from '@/mocks/server'
import { captureRequests } from '@/test/mock-helpers'
import { renderWithRouter } from '@/test/utils'

const STATUS_PATH = '/api/v1/system/bilibili-cookie'
const TESTS_PATH = '/api/v1/system/bilibili-cookie/tests'

function statusBody(overrides: Partial<BilibiliCookieStatus> = {}): BilibiliCookieStatus {
  return {
    configured: true,
    updated_at: '2026-07-20T08:00:00Z',
    sessdata_expires_at: '2027-01-15T00:00:00Z',
    expired: false,
    masked_sessdata: 'dd7d21e...IIEC',
    cookie_count: 3,
    ...overrides,
  }
}

const EMPTY_STATUS: BilibiliCookieStatus = {
  configured: false,
  updated_at: null,
  sessdata_expires_at: null,
  expired: false,
  masked_sessdata: '',
  cookie_count: 0,
}

function useStatus(body: BilibiliCookieStatus) {
  server.use(http.get(STATUS_PATH, () => HttpResponse.json(body)))
}

/**
 * 设置页 B 站 Cookie 区块：状态呈现、保存、删除与可用性检测。
 * 完整 Cookie 只出现在保存请求体中，任何渲染/缓存都不得回显。
 */
describe('BilibiliCookieSection', () => {
  it('未配置时显示空状态与保存入口', async () => {
    useStatus(statusBody({ configured: false, updated_at: null, sessdata_expires_at: null, masked_sessdata: '', cookie_count: 0 }))
    renderWithRouter('/settings')

    expect(await screen.findByText('尚未保存 B 站 Cookie')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '保存 Cookie' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '检测可用性' })).not.toBeInTheDocument()
  })

  it('已配置时显示脱敏状态、有效期与操作按钮', async () => {
    useStatus(statusBody())
    renderWithRouter('/settings')

    expect(await screen.findByText('已配置')).toBeInTheDocument()
    expect(screen.getByText('dd7d21e...IIEC')).toBeInTheDocument()
    expect(screen.getByText(/3 条/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '检测可用性' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '替换' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '删除' })).toBeInTheDocument()
  })

  it('SESSDATA 过期时显示警示', async () => {
    useStatus(statusBody({ expired: true, sessdata_expires_at: '2023-11-15T00:00:00Z' }))
    renderWithRouter('/settings')

    expect(await screen.findByText('SESSDATA 已过期')).toBeInTheDocument()
    expect(screen.getByText(/已过期，请重新获取并替换/)).toBeInTheDocument()
  })

  it('保存流程：粘贴 SESSDATA → PUT → 状态刷新，响应不含完整值', async () => {
    const user = userEvent.setup()
    let current = EMPTY_STATUS
    const { requests, stop } = captureRequests('PUT', STATUS_PATH)
    let putBody = ''
    server.use(
      http.get(STATUS_PATH, () => HttpResponse.json(current)),
      http.put(STATUS_PATH, async ({ request }) => {
        putBody = await request.text()
        current = statusBody()
        return HttpResponse.json(current)
      }),
    )
    renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '保存 Cookie' }))
    const textarea = await screen.findByLabelText(/Cookie 内容/)
    const secret = 'dd7d21e6%2C1800601560%2C0078f%2A71CjAfSECRET'
    await user.type(textarea, secret)
    await user.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => expect(putBody).toContain(secret))
    expect(JSON.parse(putBody)).toEqual({ content: secret })
    // 保存成功后回到已配置视图（MSW 返回脱敏状态）。
    expect(await screen.findByText('已配置')).toBeInTheDocument()
    expect(requests.length).toBeGreaterThan(0)
    stop()
  })

  it('保存失败时显示 code-first 错误提示', async () => {
    const user = userEvent.setup()
    useStatus(statusBody({ configured: false, updated_at: null, sessdata_expires_at: null, masked_sessdata: '', cookie_count: 0 }))
    server.use(
      http.put(STATUS_PATH, () =>
        HttpResponse.json(
          {
            type: 'https://ai-study-tutor.local/problems/bad_request',
            title: '请求无效',
            status: 400,
            detail: '内容无法识别。',
            code: 'bad_request',
            request_id: 'req-cookie-1',
          },
          { status: 400 },
        ),
      ),
    )
    renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '保存 Cookie' }))
    await user.type(await screen.findByLabelText(/Cookie 内容/), 'not-a-cookie value')
    await user.click(screen.getByRole('button', { name: '保存' }))

    expect(await screen.findByText(/无法保存 B 站 Cookie/)).toBeInTheDocument()
  })

  it('删除流程：确认对话框 → DELETE → 回到空状态', async () => {
    const user = userEvent.setup()
    let current = statusBody()
    const { requests, stop } = captureRequests('DELETE', STATUS_PATH)
    server.use(
      http.get(STATUS_PATH, () => HttpResponse.json(current)),
      http.delete(STATUS_PATH, () => {
        current = EMPTY_STATUS
        return new HttpResponse(null, { status: 204 })
      }),
    )
    renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '删除' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: '删除' }))

    await waitFor(() => expect(requests.length).toBe(1))
    expect(await screen.findByText('尚未保存 B 站 Cookie')).toBeInTheDocument()
    stop()
  })

  it('检测可用性：创建任务并跳转到任务详情', async () => {
    const user = userEvent.setup()
    useStatus(statusBody())
    const jobId = '019b3e60-7c9a-7f4e-8a4b-2f4d6c8a0e22'
    const { requests, stop } = captureRequests('POST', TESTS_PATH)
    server.use(
      http.post(TESTS_PATH, () =>
        HttpResponse.json(
          {
            job: {
              id: jobId,
              kind: 'bilibili_cookie_test',
              status: 'queued',
              stage: 'queued',
              progress: 0,
              attempt_count: 0,
              max_attempts: 3,
              retryable: true,
              cancel_requested: false,
              error: null,
              created_at: '2026-07-26T08:00:00Z',
              updated_at: '2026-07-26T08:00:00Z',
            },
          },
          { status: 202 },
        ),
      ),
    )
    const { router } = renderWithRouter('/settings')

    await user.click(await screen.findByRole('button', { name: '检测可用性' }))

    await waitFor(() => expect(requests.length).toBe(1))
    expect(requests[0].key).toBeTruthy()
    await waitFor(() => expect(router.state.location.pathname).toBe(`/jobs/${jobId}`))
    stop()
  })
})
