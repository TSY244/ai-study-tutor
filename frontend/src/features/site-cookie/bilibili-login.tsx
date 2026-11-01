import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckCircle2, LoaderCircle, QrCode, RefreshCw } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

import { cancelJob } from '@/api/generated/endpoints/jobs/jobs'
import { createBilibiliLogin, pollBilibiliLogin } from '@/api/generated/endpoints/system/system'
import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { queryKeys } from '@/api/query-client'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { FormAlert } from '@/components/ui/form-alert'

const terminalStates = new Set(['logged_in', 'expired', 'failed', 'cancelled'])

export function BilibiliLogin({ disabled = false }: { disabled?: boolean }) {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [loginId, setLoginId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const attempt = useRef(0)
  const completed = useRef<string | null>(null)
  const createKey = useRef<string | null>(null)
  const poll = useQuery({
    queryKey: ['system', 'bilibili-login', loginId],
    queryFn: ({ signal }) => pollBilibiliLogin(loginId!, { signal, cache: 'no-store' }),
    enabled: open && loginId !== null,
    retry: false,
    gcTime: 0,
    refetchInterval: (query) =>
      query.state.error || terminalStates.has(query.state.data?.state ?? '') ? false : 1500,
  })

  useEffect(() => {
    if (poll.data?.state === 'logged_in' && loginId && completed.current !== loginId) {
      completed.current = loginId
      void queryClient.invalidateQueries({ queryKey: queryKeys.bilibiliCookie() })
    }
  }, [poll.data?.state, loginId, queryClient])

  const cancel = async (id: string) => {
    try {
      await cancelJob(id, { headers: { 'Idempotency-Key': crypto.randomUUID() } })
    } catch (error) {
      // A confirmation can finish between the last poll and cancellation.
      if (!isApiProblemError(error) || error.status !== 422) throw error
    }
  }

  const start = async () => {
    setOpen(true)
    setCreating(true)
    setActionError(null)
    const currentAttempt = ++attempt.current
    try {
      if (loginId && !terminalStates.has(poll.data?.state ?? '')) await cancel(loginId)
      setLoginId(null)
      createKey.current ??= crypto.randomUUID()
      const accepted = await createBilibiliLogin({
        headers: { 'Idempotency-Key': createKey.current },
      })
      createKey.current = null
      if (attempt.current !== currentAttempt) {
        await cancel(accepted.job.id)
        return
      }
      setLoginId(accepted.job.id)
    } catch (error) {
      setActionError(error)
    } finally {
      setCreating(false)
    }
  }

  const close = async () => {
    ++attempt.current
    try {
      if (loginId && !terminalStates.has(poll.data?.state ?? '')) await cancel(loginId)
      setOpen(false)
      setLoginId(null)
    } catch (error) {
      setActionError(error)
    }
  }

  const data = poll.data
  const error = actionError ?? poll.error
  const presentation = error ? presentProblem(error) : null
  const done = data?.state === 'logged_in'
  const expired = data?.state === 'expired'
  const stopped = terminalStates.has(data?.state ?? '')

  return (
    <>
      <Button size="sm" disabled={disabled || creating} onClick={() => void start()}>
        <QrCode aria-hidden="true" className="h-4 w-4" />
        扫码登录
      </Button>
      <Dialog open={open} onOpenChange={(next) => !next && void close()}>
        <DialogContent
          title="扫码登录哔哩哔哩"
          description="使用哔哩哔哩 App 扫码，并在手机上确认。登录成功后自动保存，无需复制 Cookie。"
          className="max-w-md"
        >
          <div className="space-y-4">
            <div className="flex min-h-64 flex-col items-center justify-center rounded-md border border-border bg-surface-sunken p-5">
              {done ? (
                <CheckCircle2 aria-hidden="true" className="h-14 w-14 text-ok" />
              ) : data?.qr_image_data_url && !stopped ? (
                <img
                  src={data.qr_image_data_url}
                  alt="哔哩哔哩登录二维码"
                  width={224}
                  height={224}
                  className="rounded-md bg-white p-2"
                />
              ) : stopped || presentation ? (
                <QrCode aria-hidden="true" className="h-14 w-14 text-text-subtle" />
              ) : (
                <LoaderCircle
                  aria-label="正在生成二维码"
                  className="h-8 w-8 animate-spin text-text-muted"
                />
              )}
              <p
                role="status"
                aria-live="polite"
                className="mt-4 text-center text-sm font-medium text-text"
              >
                {creating ? '正在生成登录二维码…' : (data?.message ?? '正在等待登录服务…')}
              </p>
              {data?.expires_at && !stopped ? (
                <p className="mt-1 text-xs text-text-subtle">
                  二维码 3 分钟内有效，过期后可重新生成。
                </p>
              ) : null}
            </div>
            {presentation ? (
              <FormAlert title={presentation.title} description={presentation.message} />
            ) : null}
            <div className="flex justify-end gap-2">
              <Button variant="secondary" size="sm" onClick={() => void close()}>
                {done ? '完成' : '取消登录'}
              </Button>
              {!done ? (
                <Button size="sm" disabled={creating} onClick={() => void start()}>
                  <RefreshCw aria-hidden="true" className="h-4 w-4" />
                  {expired ? '重新生成二维码' : '刷新二维码'}
                </Button>
              ) : null}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
