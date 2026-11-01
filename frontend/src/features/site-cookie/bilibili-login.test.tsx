import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { BilibiliLoginStatus } from '@/api/generated/model'
import { server } from '@/mocks/server'
import { renderWithRouter } from '@/test/utils'

const PATH = '/api/v1/system/bilibili-cookie/logins'
const ID = '019b3e60-7c9a-7f4e-8a4b-2f4d6c8a0e33'
const QR = 'data:image/png;base64,iVBORw0KGgo='

function loginMock(state: () => BilibiliLoginStatus) {
  server.use(
    http.post(PATH, ({ request }) => {
      expect(request.headers.get('Idempotency-Key')).toBeTruthy()
      return HttpResponse.json(
        { job: { id: ID, kind: 'bilibili_login', status: 'queued' } },
        { status: 202 },
      )
    }),
    http.post(`${PATH}/${ID}/poll`, () => HttpResponse.json(state())),
  )
}

describe('Bilibili QR login', () => {
  it('shows a QR code and waits for mobile confirmation before updating login status', async () => {
    const user = userEvent.setup()
    let phase: BilibiliLoginStatus['state'] = 'waiting_scan'
    let statusReads = 0
    loginMock(() => ({
      state: phase,
      message: phase === 'logged_in' ? '登录成功，已自动保存。' : '请用哔哩哔哩 App 扫码登录。',
      qr_image_data_url: phase === 'logged_in' ? undefined : QR,
      expires_at: '2026-11-01T07:03:00Z',
    }))
    server.use(
      http.get('/api/v1/system/bilibili-cookie', () => {
        statusReads++
        return HttpResponse.json({
          configured: phase === 'logged_in',
          updated_at: null,
          sessdata_expires_at: null,
          expired: false,
          masked_sessdata: '',
          cookie_count: 0,
        })
      }),
    )
    renderWithRouter('/settings')
    await user.click(await screen.findByRole('button', { name: '扫码登录' }))
    const dialog = await screen.findByRole('dialog', { name: '扫码登录哔哩哔哩' })
    expect(await within(dialog).findByRole('img', { name: '哔哩哔哩登录二维码' })).toHaveAttribute(
      'src',
      QR,
    )
    expect(within(dialog).queryByRole('button', { name: '完成' })).not.toBeInTheDocument()
    const readsBeforeConfirmation = statusReads
    phase = 'logged_in'
    expect(
      await within(dialog).findByRole('button', { name: '完成' }, { timeout: 4000 }),
    ).toBeVisible()
    await waitFor(() => expect(statusReads).toBeGreaterThan(readsBeforeConfirmation))
    expect(within(dialog).queryByRole('img')).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: '完成' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('offers a new QR code after expiry and cancels an active login when closed', async () => {
    const user = userEvent.setup()
    let phase: BilibiliLoginStatus['state'] = 'expired'
    let cancels = 0
    loginMock(() => ({
      state: phase,
      message: phase === 'expired' ? '二维码已过期，请重新生成。' : '等待扫码',
      qr_image_data_url: QR,
    }))
    server.use(
      http.post(`/api/v1/jobs/${ID}/cancel`, () => {
        cancels++
        return HttpResponse.json({ id: ID, status: 'cancel_requested' }, { status: 202 })
      }),
    )
    renderWithRouter('/settings')
    await user.click(await screen.findByRole('button', { name: '扫码登录' }))
    const dialog = await screen.findByRole('dialog')
    expect(await within(dialog).findByRole('button', { name: '重新生成二维码' })).toBeVisible()
    phase = 'waiting_scan'
    await user.click(within(dialog).getByRole('button', { name: '重新生成二维码' }))
    expect(await within(dialog).findByRole('img', { name: '哔哩哔哩登录二维码' })).toBeVisible()
    await user.click(within(dialog).getByRole('button', { name: '取消登录' }))
    await waitFor(() => expect(cancels).toBe(1))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
