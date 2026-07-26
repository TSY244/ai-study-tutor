import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { PlugZap } from 'lucide-react'
import { useState, useSyncExternalStore } from 'react'

import { getDiagnostics, subscribeDiagnostics } from '@/api/diagnostics'
import {
  createClaudeConnectivityTest,
  getSystemStatus,
} from '@/api/generated/endpoints/system/system'
import {
  getJobStreamStatus,
  subscribeJobStreamStatus,
  type JobStreamState,
} from '@/api/job-stream-status'
import { isApiProblemError } from '@/api/problem'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ErrorState } from '@/components/ui/error-state'
import { FormAlert } from '@/components/ui/form-alert'
import { PageHeader } from '@/components/ui/page-header'
import { Skeleton } from '@/components/ui/skeleton'
import { ClaudeGatewaySection } from '@/features/claude-gateway/claude-gateway-section'
import { BilibiliCookieSection } from '@/features/site-cookie/bilibili-cookie-section'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'
import { presentProblem } from '@/api/problem-messages'
import { systemStatusMeta } from '@/lib/status'

/**
 * 设置：Claude 网关、连接状态、系统信息与本地诊断。
 * 安全边界：完整 Key 只在新增/替换表单中短暂存在并直送本机 API；
 * 前端存储、查询缓存、响应、任务与诊断都不得保存完整 Key。
 */
export const Route = createFileRoute('/_shell/settings')({
  component: SettingsPage,
})

function SettingsPage() {
  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <PageHeader title="设置" description="管理本地 Claude 网关、连接状态与诊断信息。" />
      <ClaudeSection />
      <ClaudeGatewaySection />
      <BilibiliCookieSection />
      <SystemSection />
      {import.meta.env.DEV ? <StreamDiagnosticsSection /> : null}
      <DiagnosticsSection />
    </div>
  )
}

function Section({
  id,
  title,
  children,
}: {
  id: string
  title: string
  children: React.ReactNode
}) {
  return (
    <section aria-labelledby={id} className="rounded-md border border-border bg-surface p-4">
      <h2 id={id} className="text-base font-medium text-text">
        {title}
      </h2>
      <div className="mt-3">{children}</div>
    </section>
  )
}

function useSystemStatus() {
  return useQuery({
    queryKey: queryKeys.systemStatus(),
    queryFn: () => getSystemStatus(),
  })
}

function ClaudeSection() {
  const statusQuery = useSystemStatus()
  return (
    <Section id="claude-heading" title="Claude 网关连接">
      {statusQuery.isPending ? <Skeleton className="h-4 w-40" /> : null}
      {statusQuery.isError ? (
        <ErrorState error={statusQuery.error} onRetry={() => void statusQuery.refetch()} />
      ) : null}
      {statusQuery.isSuccess ? (
        <div className="space-y-2 text-sm">
          <p className="flex items-center gap-2">
            <span className="text-text-muted">状态</span>
            <Badge variant={systemStatusMeta(statusQuery.data.claude.status).variant}>
              {systemStatusMeta(statusQuery.data.claude.status).label}
            </Badge>
          </p>
          {statusQuery.data.claude.message ? (
            <p className="text-text-muted">{statusQuery.data.claude.message}</p>
          ) : null}
        </div>
      ) : null}
      <ClaudeConnectivityTestButton />
      <p className="mt-3 border-t border-border pt-3 text-sm text-text-muted">
        此测试通过当前网关路由执行，会按优先级、权重与故障转移策略选择 Key。 旧单 Key
        配置仅在网关没有启用 Key 时回退使用。
      </p>
    </Section>
  )
}

const CLAUDE_CONNECTIVITY_INTENT_SCOPE = 'claude-connectivity-test'

function ClaudeConnectivityTestButton() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [error, setError] = useState<unknown>(null)
  const testMutation = useMutation({
    mutationFn: () =>
      createClaudeConnectivityTest({
        headers: { 'Idempotency-Key': beginIdempotencyIntent(CLAUDE_CONNECTIVITY_INTENT_SCOPE) },
      }),
    onSuccess: async (accepted) => {
      resolveIdempotencyIntent(CLAUDE_CONNECTIVITY_INTENT_SCOPE)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.systemStatus() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.jobs() }),
      ])
      await navigate({ to: '/jobs/$jobId', params: { jobId: accepted.job.id } })
    },
    onError: (mutationError) => {
      if (isApiProblemError(mutationError) && mutationError.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(CLAUDE_CONNECTIVITY_INTENT_SCOPE)
      }
      setError(mutationError)
    },
  })

  const presentation = error ? presentProblem(error) : null

  return (
    <div className="mt-3 space-y-2">
      <Button
        size="sm"
        disabled={testMutation.isPending}
        onClick={() => {
          setError(null)
          testMutation.mutate()
        }}
      >
        <PlugZap aria-hidden="true" className="h-4 w-4" />
        {testMutation.isPending ? '创建测试任务中…' : '测试 Claude 网关连接'}
      </Button>
      {presentation ? (
        <FormAlert
          title={presentation.title}
          description={presentation.message}
          code={isApiProblemError(error) ? error.code : undefined}
          requestId={isApiProblemError(error) ? error.requestId : undefined}
        />
      ) : null}
    </div>
  )
}

function SystemSection() {
  const statusQuery = useSystemStatus()
  return (
    <Section id="system-heading" title="系统信息">
      {statusQuery.isPending ? <Skeleton className="h-4 w-56" /> : null}
      {statusQuery.isSuccess ? (
        <dl className="space-y-2 text-sm">
          <div className="flex items-center justify-between gap-4">
            <dt className="text-text-muted">版本</dt>
            <dd className="font-mono text-text">{statusQuery.data.version}</dd>
          </div>
          <div className="flex items-center justify-between gap-4">
            <dt className="text-text-muted">维护模式</dt>
            <dd>
              {statusQuery.data.maintenance.enabled ? (
                <Badge variant="warn">已开启</Badge>
              ) : (
                <Badge variant="ok">未开启</Badge>
              )}
            </dd>
          </div>
          <div className="flex items-center justify-between gap-4">
            <dt className="text-text-muted">本地存储</dt>
            <dd>
              <Badge variant={systemStatusMeta(statusQuery.data.storage.status).variant}>
                {systemStatusMeta(statusQuery.data.storage.status).label}
              </Badge>
            </dd>
          </div>
        </dl>
      ) : null}
    </Section>
  )
}

const STREAM_STATE_META: Record<
  JobStreamState,
  { label: string; variant: 'ok' | 'warn' | 'neutral' | 'info' }
> = {
  idle: { label: '未启动', variant: 'neutral' },
  connecting: { label: '连接中', variant: 'info' },
  open: { label: '已连接', variant: 'ok' },
  reconnecting: { label: '重连中', variant: 'warn' },
  closed: { label: '已关闭', variant: 'neutral' },
}

/**
 * 任务事件流诊断（F6，仅开发模式）：连接状态、游标、心跳、重连与
 * 事件计数、最近 resync 对账。只读元数据，不展示事件正文。
 */
function StreamDiagnosticsSection() {
  const status = useSyncExternalStore(subscribeJobStreamStatus, getJobStreamStatus)
  const stateMeta = STREAM_STATE_META[status.state]
  const eventEntries = Object.entries(status.eventCounts)

  return (
    <Section id="stream-heading" title="任务事件流（开发诊断）">
      <dl className="space-y-2 text-sm">
        <div className="flex items-center justify-between gap-4">
          <dt className="text-text-muted">连接状态</dt>
          <dd>
            <Badge variant={stateMeta.variant}>{stateMeta.label}</Badge>
            {status.reconnectAttempts > 0 && status.state === 'reconnecting' ? (
              <span className="ml-2 text-xs text-text-subtle">
                第 {status.reconnectAttempts} 次重连
              </span>
            ) : null}
          </dd>
        </div>
        <div className="flex items-center justify-between gap-4">
          <dt className="text-text-muted">事件游标</dt>
          <dd className="font-mono text-text">{status.lastEventId ?? '—'}</dd>
        </div>
        <div className="flex items-center justify-between gap-4">
          <dt className="text-text-muted">最近心跳</dt>
          <dd className="text-text">{status.lastHeartbeatAt ?? '—'}</dd>
        </div>
        <div className="flex items-center justify-between gap-4">
          <dt className="text-text-muted">最近事件</dt>
          <dd className="text-text">{status.lastEventAt ?? '—'}</dd>
        </div>
        <div className="flex items-center justify-between gap-4">
          <dt className="text-text-muted">最近断档对账</dt>
          <dd className="text-text">{status.lastResyncAt ?? '—'}</dd>
        </div>
        {status.lastError ? (
          <div className="flex items-center justify-between gap-4">
            <dt className="text-text-muted">最近错误</dt>
            <dd className="text-right text-danger">{status.lastError}</dd>
          </div>
        ) : null}
      </dl>
      {eventEntries.length > 0 ? (
        <p className="mt-3 border-t border-border pt-3 text-xs text-text-subtle">
          事件计数：
          {eventEntries.map(([event, count]) => `${event} ×${count}`).join('，')}
        </p>
      ) : null}
    </Section>
  )
}

/** 最近请求诊断（F0 环形缓冲，仅元数据）：排查问题时提供 Request ID。 */
function DiagnosticsSection() {
  const diagnostics = useSyncExternalStore(subscribeDiagnostics, getDiagnostics)

  return (
    <Section id="diagnostics-heading" title="最近请求诊断">
      {diagnostics.length === 0 ? (
        <p className="text-sm text-text-muted">
          暂无记录。请求出错时会在这里保留最近 50 条元数据（不含请求/响应内容）。
        </p>
      ) : (
        <ul className="divide-y divide-border text-sm">
          {diagnostics.slice(0, 8).map((entry, index) => (
            <li key={`${entry.requestId}-${index}`} className="py-2">
              <div className="flex items-center justify-between gap-3">
                <span className="font-mono text-xs break-all text-text">{entry.requestId}</span>
                <Badge variant={entry.status >= 500 ? 'danger' : 'warn'}>
                  {entry.code ?? `HTTP ${entry.status}`}
                </Badge>
              </div>
              <p className="mt-0.5 text-xs text-text-subtle">
                {entry.method} {entry.url} · {entry.at}
              </p>
            </li>
          ))}
        </ul>
      )}
    </Section>
  )
}
