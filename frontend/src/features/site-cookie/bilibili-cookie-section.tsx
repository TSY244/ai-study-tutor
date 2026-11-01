import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Cookie, Pencil, PlugZap, Trash2 } from 'lucide-react'
import { useState } from 'react'

import {
  createBilibiliCookieTest,
  deleteBilibiliCookie,
  getBilibiliCookieStatus,
  saveBilibiliCookie,
} from '@/api/generated/endpoints/system/system'
import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { ErrorState } from '@/components/ui/error-state'
import { FormAlert } from '@/components/ui/form-alert'
import { Skeleton } from '@/components/ui/skeleton'
import { BilibiliCookieDialog } from '@/features/site-cookie/bilibili-cookie-dialog'
import { BilibiliLogin } from '@/features/site-cookie/bilibili-login'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

const TEST_INTENT_SCOPE = 'bilibili-cookie-test'

/**
 * B 站 Cookie 管理：保存/替换/删除与可用性检测。
 * 安全边界：完整 Cookie 只在保存表单中短暂存在并直送本机 API；
 * 前端存储、查询缓存、响应与诊断都不得保存完整 Cookie。
 */
export function BilibiliCookieSection() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const statusQuery = useQuery({
    queryKey: queryKeys.bilibiliCookie(),
    queryFn: () => getBilibiliCookieStatus(),
  })
  const [dialogOpen, setDialogOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [busyAction, setBusyAction] = useState<'test' | 'delete' | null>(null)
  const [actionError, setActionError] = useState<unknown>(null)

  const data = statusQuery.data

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.bilibiliCookie() })
  }

  const save = async (content: string) => {
    setActionError(null)
    await saveBilibiliCookie({ content })
    await refresh()
  }

  const runTest = async () => {
    setBusyAction('test')
    setActionError(null)
    try {
      const accepted = await createBilibiliCookieTest({
        headers: { 'Idempotency-Key': beginIdempotencyIntent(TEST_INTENT_SCOPE) },
      })
      resolveIdempotencyIntent(TEST_INTENT_SCOPE)
      await Promise.all([refresh(), queryClient.invalidateQueries({ queryKey: queryKeys.jobs() })])
      await navigate({ to: '/jobs/$jobId', params: { jobId: accepted.job.id } })
    } catch (error) {
      if (isApiProblemError(error) && error.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(TEST_INTENT_SCOPE)
      }
      setActionError(error)
    } finally {
      setBusyAction(null)
    }
  }

  const remove = async () => {
    setBusyAction('delete')
    setActionError(null)
    try {
      await deleteBilibiliCookie()
      setConfirmDelete(false)
      await refresh()
    } catch (error) {
      setActionError(error)
    } finally {
      setBusyAction(null)
    }
  }

  const presentation = actionError ? presentProblem(actionError) : null

  return (
    <section
      aria-labelledby="bilibili-cookie-heading"
      className="rounded-md border border-border bg-surface p-4"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 id="bilibili-cookie-heading" className="text-base font-medium text-text">
            B 站 Cookie
          </h2>
          <p className="mt-1 text-sm text-text-muted">
            用于导入 bilibili 视频链接时获取登录后才可见的字幕。保存后立即生效，无需重启。
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <BilibiliLogin disabled={busyAction !== null} />
          {data?.configured ? (
            <>
              <Button
                variant="secondary"
                size="sm"
                disabled={busyAction !== null}
                onClick={() => void runTest()}
              >
                <PlugZap aria-hidden="true" className="h-4 w-4" />
                {busyAction === 'test' ? '创建检测任务中…' : '检测可用性'}
              </Button>
              <Button
                variant="secondary"
                size="sm"
                disabled={busyAction !== null}
                onClick={() => {
                  setActionError(null)
                  setDialogOpen(true)
                }}
              >
                <Pencil aria-hidden="true" className="h-4 w-4" />
                替换
              </Button>
              <Button
                variant="danger"
                size="sm"
                disabled={busyAction !== null}
                onClick={() => {
                  setActionError(null)
                  setConfirmDelete(true)
                }}
              >
                <Trash2 aria-hidden="true" className="h-4 w-4" />
                删除
              </Button>
            </>
          ) : null}
        </div>
      </div>

      {statusQuery.isPending ? (
        <div className="mt-4 space-y-2">
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-4 w-64" />
        </div>
      ) : null}
      {statusQuery.isError ? (
        <div className="mt-4">
          <ErrorState error={statusQuery.error} onRetry={() => void statusQuery.refetch()} />
        </div>
      ) : null}

      {data ? (
        <div className="mt-4">
          {data.configured ? (
            <dl className="grid gap-x-5 gap-y-2 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-text-subtle">状态</dt>
                <dd className="mt-0.5">
                  {data.expired ? (
                    <Badge variant="danger">SESSDATA 已过期</Badge>
                  ) : (
                    <Badge variant="ok">已配置</Badge>
                  )}
                </dd>
              </div>
              <div>
                <dt className="text-text-subtle">SESSDATA（脱敏）</dt>
                <dd className="mt-0.5 font-mono text-text-muted">{data.masked_sessdata}</dd>
              </div>
              <div>
                <dt className="text-text-subtle">有效期至</dt>
                <dd className="mt-0.5 text-text-muted">
                  {data.sessdata_expires_at ? formatDate(data.sessdata_expires_at) : '未记录'}
                  {data.expired ? '（已过期，请重新获取并替换）' : ''}
                </dd>
              </div>
              <div>
                <dt className="text-text-subtle">条目数 / 最近保存</dt>
                <dd className="mt-0.5 text-text-muted">
                  {data.cookie_count} 条 · {data.updated_at ? formatDateTime(data.updated_at) : '—'}
                </dd>
              </div>
            </dl>
          ) : (
            <div className="rounded-md border border-dashed border-border-strong px-4 py-6 text-center">
              <p className="flex items-center justify-center gap-2 text-sm font-medium text-text">
                <Cookie aria-hidden="true" className="h-4 w-4" />
                尚未保存 B 站 Cookie
              </p>
              <p className="mt-1 text-sm text-text-muted">
                不保存也能导入公开视频；需要会员/登录可见字幕时再配置。
              </p>
              <Button
                size="sm"
                className="mt-3"
                onClick={() => {
                  setActionError(null)
                  setDialogOpen(true)
                }}
              >
                保存 Cookie
              </Button>
            </div>
          )}
        </div>
      ) : null}

      {presentation ? (
        <div className="mt-3">
          <FormAlert
            title={presentation.title}
            description={presentation.message}
            code={isApiProblemError(actionError) ? actionError.code : undefined}
            requestId={isApiProblemError(actionError) ? actionError.requestId : undefined}
          />
        </div>
      ) : null}

      <p className="mt-4 border-t border-border pt-3 text-xs text-text-subtle">
        完整 Cookie 只在保存时发送一次并写入本机部署（0600）；界面与接口只显示脱敏视图。
        保存时仅保留 bilibili.com 域名条目。
      </p>

      <BilibiliCookieDialog
        open={dialogOpen}
        replacing={data?.configured ?? false}
        onOpenChange={setDialogOpen}
        onSave={save}
      />
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={(open) => {
          if (!open) setConfirmDelete(false)
        }}
        title="删除 B 站 Cookie"
        description="删除后，导入 bilibili 链接时将无法获取登录可见的字幕。此操作不可恢复。"
        confirmLabel="删除"
        danger
        busy={busyAction === 'delete'}
        onConfirm={() => void remove()}
      />
    </section>
  )
}

function formatDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.valueOf())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date)
}

function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.valueOf())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}
