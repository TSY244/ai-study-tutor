import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { KeyRound, Plus, Route as RouteIcon, Save, ShieldCheck } from 'lucide-react'
import { useState } from 'react'

import {
  createClaudeKey,
  createClaudeKeyTest,
  deleteClaudeKey,
  listClaudeKeys,
  saveClaudeKeyToClaudeCodeSettings,
  updateClaudeKey,
} from '@/api/generated/endpoints/system/system'
import type {
  ClaudeKey,
  ClaudeKeyCreate,
  ClaudeKeyStatus,
  ClaudeKeyUpdate,
} from '@/api/generated/model'
import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { queryKeys } from '@/api/query-client'
import { Badge, type BadgeVariant } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { ErrorState } from '@/components/ui/error-state'
import { FormAlert } from '@/components/ui/form-alert'
import { Skeleton } from '@/components/ui/skeleton'
import { ClaudeKeyDialog } from '@/features/claude-gateway/claude-key-dialog'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

const STATUS_META: Record<ClaudeKeyStatus, { label: string; variant: BadgeVariant }> = {
  unknown: { label: '待检测', variant: 'neutral' },
  healthy: { label: '健康', variant: 'ok' },
  cooling_down: { label: '冷却中', variant: 'warn' },
  auth_failed: { label: '认证失败', variant: 'danger' },
  disabled: { label: '已停用', variant: 'neutral' },
  testing: { label: '检测中', variant: 'info' },
}

type ConfirmAction = { kind: 'toggle' | 'delete' | 'claude-code-settings'; keyRecord: ClaudeKey }

export function ClaudeGatewaySection() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const keysQuery = useQuery({ queryKey: queryKeys.claudeKeys(), queryFn: () => listClaudeKeys() })
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingKey, setEditingKey] = useState<ClaudeKey | null>(null)
  const [confirmAction, setConfirmAction] = useState<ConfirmAction | null>(null)
  const [busyAction, setBusyAction] = useState<string | null>(null)
  const [actionError, setActionError] = useState<unknown>(null)
  const [claudeCodeSavedKeyName, setClaudeCodeSavedKeyName] = useState<string | null>(null)

  const data = keysQuery.data
  const items = data?.items ?? []
  const enabledCount = items.filter((key) => key.enabled).length
  const healthyCount = items.filter((key) => key.status === 'healthy').length
  const disabledCount = items.filter((key) => !key.enabled).length
  const latestSuccess = items
    .map((key) => key.last_success_at)
    .filter((value): value is string => value !== null)
    .sort()
    .at(-1)

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.claudeKeys() })
  }

  const saveKey = async (payload: ClaudeKeyCreate | ClaudeKeyUpdate) => {
    setActionError(null)
    if (editingKey) {
      await updateClaudeKey(editingKey.id, payload as ClaudeKeyUpdate)
    } else {
      const scope = 'claude-gateway-key-create'
      try {
        await createClaudeKey(payload as ClaudeKeyCreate, {
          headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) },
        })
        resolveIdempotencyIntent(scope)
      } catch (error) {
        if (isApiProblemError(error) && error.code === 'idempotency_conflict') {
          rotateIdempotencyIntent(scope)
        }
        throw error
      }
    }
    await refresh()
  }

  const testKey = async (keyRecord: ClaudeKey) => {
    const scope = `claude-gateway-key-test:${keyRecord.id}`
    setBusyAction(`test:${keyRecord.id}`)
    setActionError(null)
    try {
      const accepted = await createClaudeKeyTest(keyRecord.id, {
        headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) },
      })
      resolveIdempotencyIntent(scope)
      await Promise.all([refresh(), queryClient.invalidateQueries({ queryKey: queryKeys.jobs() })])
      await navigate({ to: '/jobs/$jobId', params: { jobId: accepted.job.id } })
    } catch (error) {
      if (isApiProblemError(error) && error.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(scope)
      }
      setActionError(error)
    } finally {
      setBusyAction(null)
    }
  }

  const confirm = async () => {
    if (!confirmAction) return
    const { keyRecord, kind } = confirmAction
    setBusyAction(`${kind}:${keyRecord.id}`)
    setActionError(null)
    try {
      switch (kind) {
        case 'delete':
          await deleteClaudeKey(keyRecord.id)
          await refresh()
          break
        case 'toggle':
          await updateClaudeKey(keyRecord.id, { enabled: !keyRecord.enabled })
          await refresh()
          break
        case 'claude-code-settings':
          await saveClaudeKeyToClaudeCodeSettings(keyRecord.id)
          setClaudeCodeSavedKeyName(keyRecord.name)
          break
      }
      setConfirmAction(null)
    } catch (error) {
      setActionError(error)
    } finally {
      setBusyAction(null)
    }
  }

  const presentation = actionError ? presentProblem(actionError) : null
  const writeEnabled = data?.write_enabled ?? false

  return (
    <section
      aria-labelledby="claude-gateway-heading"
      className="rounded-md border border-border bg-surface p-4"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 id="claude-gateway-heading" className="text-base font-medium text-text">
            Claude 多 Key 网关
          </h2>
          <p className="mt-1 text-sm text-text-muted">
            仅支持 Anthropic Claude。优先级越小越先使用，同优先级按权重轮询并自动故障转移。
          </p>
        </div>
        <Button
          size="sm"
          disabled={!writeEnabled}
          onClick={() => {
            setEditingKey(null)
            setDialogOpen(true)
          }}
        >
          <Plus aria-hidden="true" className="h-4 w-4" />
          添加 Key
        </Button>
      </div>

      {keysQuery.isPending ? (
        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          <Skeleton className="h-20" />
          <Skeleton className="h-20" />
        </div>
      ) : null}
      {keysQuery.isError ? (
        <div className="mt-4">
          <ErrorState error={keysQuery.error} onRetry={() => void keysQuery.refetch()} />
        </div>
      ) : null}

      {data ? (
        <>
          <div
            className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-4"
            aria-label="Claude 网关摘要"
          >
            <SummaryTile icon={<KeyRound aria-hidden="true" className="h-4 w-4" />}>
              已启用 {enabledCount}
            </SummaryTile>
            <SummaryTile icon={<ShieldCheck aria-hidden="true" className="h-4 w-4" />}>
              健康 {healthyCount} · 已停用 {disabledCount}
            </SummaryTile>
            <SummaryTile icon={<RouteIcon aria-hidden="true" className="h-4 w-4" />}>
              优先级故障转移
            </SummaryTile>
            <SummaryTile>
              最近成功 {latestSuccess ? formatDateTime(latestSuccess) : '尚无记录'}
            </SummaryTile>
          </div>

          {data.legacy_fallback ? (
            <p className="mt-3 rounded-md bg-info-bg px-3 py-2 text-sm text-info">
              当前没有启用的网关 Key，Worker 正使用旧单 Key 配置作为回退。
            </p>
          ) : null}
          {!data.write_enabled ? (
            <p className="mt-3 rounded-md bg-warn-bg px-3 py-2 text-sm text-warn">
              当前部署为只读模式。启用 CLAUDE_GATEWAY_WRITE_ENABLED 后才能新增或修改 Key。
            </p>
          ) : null}
          {claudeCodeSavedKeyName ? (
            <p role="status" className="mt-3 rounded-md bg-ok-bg px-3 py-2 text-sm text-ok">
              已将「{claudeCodeSavedKeyName}」应用到运行服务的 Claude Code。
            </p>
          ) : null}
          {presentation ? (
            <div className="mt-3">
              <FormAlert title={presentation.title} description={presentation.message} />
            </div>
          ) : null}

          {items.length === 0 ? (
            <div className="mt-4 rounded-md border border-dashed border-border-strong px-4 py-8 text-center">
              <p className="text-sm font-medium text-text">尚未添加网关 Key</p>
              <p className="mt-1 text-sm text-text-muted">
                添加第一个 Key 后，Worker 会热加载配置，无需重启。
              </p>
            </div>
          ) : (
            <div className="mt-4 space-y-3">
              {items.map((keyRecord) => (
                <KeyCard
                  key={keyRecord.id}
                  keyRecord={keyRecord}
                  writeEnabled={writeEnabled}
                  busyAction={busyAction}
                  onEdit={() => {
                    setEditingKey(keyRecord)
                    setDialogOpen(true)
                  }}
                  onTest={() => void testKey(keyRecord)}
                  onSaveToClaudeCode={() => {
                    setActionError(null)
                    setClaudeCodeSavedKeyName(null)
                    setConfirmAction({ kind: 'claude-code-settings', keyRecord })
                  }}
                  onToggle={() => setConfirmAction({ kind: 'toggle', keyRecord })}
                  onDelete={() => setConfirmAction({ kind: 'delete', keyRecord })}
                />
              ))}
            </div>
          )}
        </>
      ) : null}

      <p className="mt-4 border-t border-border pt-3 text-xs text-text-subtle">
        完整 Key 只在新增或主动替换时发送一次；列表、错误、任务与诊断只使用脱敏值和指纹。
      </p>

      <ClaudeKeyDialog
        open={dialogOpen}
        keyRecord={editingKey}
        onOpenChange={(open) => {
          setDialogOpen(open)
          if (!open) setEditingKey(null)
        }}
        onSave={saveKey}
      />
      <ConfirmDialog
        open={confirmAction?.kind === 'toggle'}
        onOpenChange={(open) => {
          if (!open) setConfirmAction(null)
        }}
        title={confirmAction?.keyRecord.enabled ? '停用 Claude Key' : '启用 Claude Key'}
        description={
          confirmAction?.keyRecord.enabled
            ? `停用「${confirmAction.keyRecord.name}」后，它会立即退出网关路由。`
            : `启用「${confirmAction?.keyRecord.name ?? ''}」后，它会重新进入网关路由。`
        }
        confirmLabel={confirmAction?.keyRecord.enabled ? '停用' : '启用'}
        busy={busyAction?.startsWith('toggle:')}
        onConfirm={() => void confirm()}
      />
      <ConfirmDialog
        open={confirmAction?.kind === 'claude-code-settings'}
        onOpenChange={(open) => {
          if (!open) setConfirmAction(null)
        }}
        title="应用到运行服务的 Claude Code"
        description={`将用「${confirmAction?.keyRecord.name ?? ''}」覆盖运行环境 ~/.claude/settings.json，并影响后续 Worker Claude Code 任务。`}
        confirmLabel="确认应用"
        busy={busyAction?.startsWith('claude-code-settings:')}
        errorMessage={
          confirmAction?.kind === 'claude-code-settings' ? presentation?.message : undefined
        }
        onConfirm={() => void confirm()}
      />
      <ConfirmDialog
        open={confirmAction?.kind === 'delete'}
        onOpenChange={(open) => {
          if (!open) setConfirmAction(null)
        }}
        title="删除 Claude Key"
        description={`将从网关配置中永久删除「${confirmAction?.keyRecord.name ?? ''}」。此操作不可恢复。`}
        confirmLabel="删除"
        danger
        busy={busyAction?.startsWith('delete:')}
        onConfirm={() => void confirm()}
      />
    </section>
  )
}

function SummaryTile({ icon, children }: { icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <p className="flex min-h-12 items-center gap-2 rounded-md bg-surface-sunken px-3 py-2 text-sm text-text">
      {icon}
      {children}
    </p>
  )
}

function KeyCard({
  keyRecord,
  writeEnabled,
  busyAction,
  onEdit,
  onTest,
  onSaveToClaudeCode,
  onToggle,
  onDelete,
}: {
  keyRecord: ClaudeKey
  writeEnabled: boolean
  busyAction: string | null
  onEdit: () => void
  onTest: () => void
  onSaveToClaudeCode: () => void
  onToggle: () => void
  onDelete: () => void
}) {
  const status = STATUS_META[keyRecord.status]
  const busy = busyAction?.endsWith(`:${keyRecord.id}`) ?? false
  return (
    <article
      aria-label={`Claude Key ${keyRecord.name}`}
      className="rounded-md border border-border p-3"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="font-medium text-text">{keyRecord.name}</h3>
            <Badge variant={status.variant}>{status.label}</Badge>
          </div>
          <p className="mt-1 font-mono text-sm text-text-muted">{keyRecord.masked_key}</p>
          <p className="font-mono text-xs text-text-subtle">{keyRecord.fingerprint}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="secondary"
            size="sm"
            aria-label={`测试${keyRecord.name}`}
            disabled={!writeEnabled || busy}
            onClick={onTest}
          >
            测试
          </Button>
          <Button
            variant="secondary"
            size="sm"
            aria-label={`应用${keyRecord.name}到运行服务的 Claude Code`}
            disabled={!writeEnabled || busy}
            onClick={onSaveToClaudeCode}
          >
            <Save aria-hidden="true" className="h-4 w-4" />
            应用到运行服务
          </Button>
          <Button
            variant="secondary"
            size="sm"
            aria-label={`编辑${keyRecord.name}`}
            disabled={!writeEnabled || busy}
            onClick={onEdit}
          >
            编辑
          </Button>
          <Button
            variant="secondary"
            size="sm"
            aria-label={`${keyRecord.enabled ? '停用' : '启用'}${keyRecord.name}`}
            disabled={!writeEnabled || busy}
            onClick={onToggle}
          >
            {keyRecord.enabled ? '停用' : '启用'}
          </Button>
          <Button
            variant="danger"
            size="sm"
            aria-label={`删除${keyRecord.name}`}
            disabled={!writeEnabled || busy}
            onClick={onDelete}
          >
            删除
          </Button>
        </div>
      </div>

      <dl className="mt-3 grid gap-x-5 gap-y-2 border-t border-border pt-3 text-xs sm:grid-cols-2 lg:grid-cols-4">
        <Metric label="模型" value={keyRecord.model} />
        <Metric label="Base URL" value={keyRecord.base_url || '默认（环境变量）'} mono />
        <Metric label="路由" value={`优先级 ${keyRecord.priority} · 权重 ${keyRecord.weight}`} />
        <Metric
          label="超时 / 冷却"
          value={`${keyRecord.timeout_seconds}s / ${keyRecord.cooldown_seconds}s`}
        />
        <Metric
          label="连续失败"
          value={`${keyRecord.consecutive_failures} / ${keyRecord.max_consecutive_failures}`}
        />
        <Metric label="最近成功" value={formatDateTime(keyRecord.last_success_at)} />
        <Metric label="最近失败" value={formatDateTime(keyRecord.last_failure_at)} />
        <Metric label="错误码" value={keyRecord.last_error_code ?? '—'} mono />
        <Metric
          label="延迟 / 冷却至"
          value={
            keyRecord.cooldown_until
              ? formatDateTime(keyRecord.cooldown_until)
              : keyRecord.last_latency_ms === null
                ? '—'
                : `${keyRecord.last_latency_ms} ms`
          }
        />
      </dl>
    </article>
  )
}

function Metric({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-text-subtle">{label}</dt>
      <dd
        className={mono ? 'mt-0.5 break-all font-mono text-text-muted' : 'mt-0.5 text-text-muted'}
      >
        {value}
      </dd>
    </div>
  )
}

function formatDateTime(value: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.valueOf())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}
