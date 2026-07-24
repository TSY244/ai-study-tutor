import { CloudOff, RefreshCw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

import type { AnswerSyncEngine, OverallSyncStatus, QuestionSyncStatus } from './answer-sync-engine'

/**
 * 自动保存状态显示：顶部全局状态条 + 每题状态文字。
 * 状态不只靠颜色——永远带文本；全局条用 aria-live="polite" 只在文本
 * 变化时播报，不打断屏幕阅读器（handoff §9）。
 */

const OVERALL_META: Record<OverallSyncStatus, { label: string; className: string }> = {
  saved: { label: '所有答案已同步', className: 'text-ok' },
  saving: { label: '正在保存…', className: 'text-text-muted' },
  unsynced: { label: '未同步：答案已保存在本机，联网后自动重试', className: 'text-danger' },
  conflict: { label: '有答案冲突待处理', className: 'text-warn' },
}

export function SyncStatusBar({
  engine,
  onOpenConflict,
}: {
  engine: AnswerSyncEngine
  onOpenConflict?: () => void
}) {
  const overall = engine.getOverallStatus()
  const meta = OVERALL_META[overall]

  return (
    <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-surface px-3 py-2">
      <p aria-live="polite" className={cn('flex items-center gap-1.5 text-sm', meta.className)}>
        {overall === 'unsynced' ? (
          <CloudOff aria-hidden="true" className="h-4 w-4 shrink-0" />
        ) : null}
        {meta.label}
      </p>
      <div className="flex items-center gap-2">
        {overall === 'conflict' && onOpenConflict ? (
          <Button variant="secondary" size="sm" onClick={onOpenConflict}>
            处理冲突
          </Button>
        ) : null}
        {overall === 'unsynced' ? (
          <Button
            variant="secondary"
            size="sm"
            onClick={() => engine.retryUnsynced()}
            className="gap-1"
          >
            <RefreshCw aria-hidden="true" className="h-3.5 w-3.5" />
            立即重试
          </Button>
        ) : null}
      </div>
    </div>
  )
}

const QUESTION_STATUS_META: Record<QuestionSyncStatus, { label: string; className: string }> = {
  saved: { label: '已保存', className: 'text-text-subtle' },
  dirty: { label: '待保存…', className: 'text-text-muted' },
  saving: { label: '保存中…', className: 'text-text-muted' },
  unsynced: { label: '未同步', className: 'text-danger' },
  conflict: { label: '冲突待处理', className: 'text-warn' },
}

/** 单题同步状态（题卡右上角）；冲突时可点击打开处理对话框。 */
export function QuestionSyncChip({
  engine,
  questionId,
  onOpenConflict,
}: {
  engine: AnswerSyncEngine
  questionId: string
  onOpenConflict?: () => void
}) {
  const status = engine.getStatus(questionId)
  const meta = QUESTION_STATUS_META[status]
  if (status === 'conflict' && onOpenConflict) {
    return (
      <button
        type="button"
        onClick={onOpenConflict}
        className={cn('rounded-sm text-xs underline underline-offset-2', meta.className)}
      >
        {meta.label}
      </button>
    )
  }
  return <span className={cn('text-xs', meta.className)}>{meta.label}</span>
}
