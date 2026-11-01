import { DeleteContentButton } from '@/features/content/delete-content-button'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FileText, Send } from 'lucide-react'
import { useEffect, useState } from 'react'

import {
  createSummaryEvaluation,
  getSummaryEvaluation,
  listSummaryEvaluations,
} from '@/api/generated/endpoints/summaries/summaries'
import type { SummaryEvaluation } from '@/api/generated/model'
import { invalidateSummaryEvaluationAccepted } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { queryKeys } from '@/api/query-client'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ErrorState } from '@/components/ui/error-state'
import { FormAlert } from '@/components/ui/form-alert'
import { Label } from '@/components/ui/label'
import { SkeletonText } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { ResourceJobs } from '@/features/jobs/resource-jobs'
import { useActiveResourceJobs } from '@/features/jobs/use-active-resource-jobs'
import {
  forgetLastEvaluationId,
  loadLastEvaluationId,
  saveLastEvaluationId,
} from '@/features/summaries/last-evaluation'
import { formatDateTime } from '@/lib/format'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

/** 契约约束：总结正文 1–50000 字。 */
const SUMMARY_MAX_LENGTH = 50_000

interface SummaryPanelProps {
  lessonId: string
}

/**
 * 学习总结：手写总结 + 提交 AI 评估。
 *
 * 服务端历史列表支持查看与删除；本地记住最近查看的评估，
 * 评估进行中由 SSE 终态对账自动刷新（F6）。
 */
export function SummaryPanel({ lessonId }: SummaryPanelProps) {
  const queryClient = useQueryClient()
  const [summary, setSummary] = useState('')
  const [submitError, setSubmitError] = useState<unknown>(null)
  const [evaluationId, setEvaluationId] = useState<string | null>(() =>
    loadLastEvaluationId(lessonId),
  )
  const scope = `summary-evaluation:${lessonId}`
  const jobsQuery = useActiveResourceJobs('summary_evaluation', evaluationId, ['summary_evaluate'])

  const evaluationQuery = useQuery({
    queryKey: queryKeys.summaryEvaluation(evaluationId ?? ''),
    queryFn: () => getSummaryEvaluation(evaluationId as string),
    enabled: evaluationId !== null,
  })

  const historyQuery = useInfiniteQuery({
    queryKey: ['summary-evaluations', lessonId],
    queryFn: ({ pageParam }) => listSummaryEvaluations(lessonId, { cursor: pageParam, limit: 20 }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) =>
      page.page_info.has_more ? (page.page_info.next_cursor ?? undefined) : undefined,
  })
  const history = historyQuery.data?.pages.flatMap((page) => page.items) ?? []
  const entries =
    evaluationQuery.isSuccess && !history.some((item) => item.id === evaluationQuery.data.id)
      ? [evaluationQuery.data, ...history]
      : history

  // 评估在服务端已不存在（如数据被清理）：忘掉本地记录，回到纯表单。
  useEffect(() => {
    if (
      evaluationQuery.isError &&
      isApiProblemError(evaluationQuery.error) &&
      evaluationQuery.error.status === 404
    ) {
      forgetLastEvaluationId(lessonId)
      setEvaluationId(null)
    }
  }, [evaluationQuery.isError, evaluationQuery.error, lessonId])

  const submitMutation = useMutation({
    mutationFn: (text: string) =>
      createSummaryEvaluation(
        lessonId,
        { summary: text },
        { headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) } },
      ),
    onSuccess: async (accepted) => {
      resolveIdempotencyIntent(scope)
      saveLastEvaluationId(lessonId, accepted.evaluation.id)
      setEvaluationId(accepted.evaluation.id)
      queryClient.setQueryData(
        queryKeys.summaryEvaluation(accepted.evaluation.id),
        accepted.evaluation,
      )
      await Promise.all([
        invalidateSummaryEvaluationAccepted(queryClient, accepted.job.id),
        queryClient.invalidateQueries({ queryKey: ['summary-evaluations', lessonId] }),
      ])
    },
    onError: (mutationError) => {
      if (isApiProblemError(mutationError) && mutationError.code === 'idempotency_conflict') {
        rotateIdempotencyIntent(scope)
      }
      setSubmitError(mutationError)
    },
  })

  const trimmed = summary.trim()
  const presentation = submitError ? presentProblem(submitError) : null

  return (
    <div className="space-y-5">
      <section aria-label="手写总结" className="space-y-3">
        <div className="space-y-1">
          <h2 className="text-base font-semibold text-text">手写总结</h2>
          <p className="text-sm text-text-muted">
            合上资料，用自己的话写下本课时的要点，然后交给 AI 评估覆盖度与误解。
          </p>
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="summary-text">总结内容</Label>
          <Textarea
            id="summary-text"
            rows={8}
            maxLength={SUMMARY_MAX_LENGTH}
            placeholder="用自己的话总结本课时学到了什么…"
            value={summary}
            onChange={(event) => setSummary(event.target.value)}
          />
          <p className="text-right text-xs text-text-subtle">
            {summary.length} / {SUMMARY_MAX_LENGTH}
          </p>
        </div>
        <Button
          size="sm"
          disabled={trimmed.length === 0 || submitMutation.isPending}
          onClick={() => {
            setSubmitError(null)
            submitMutation.mutate(trimmed)
          }}
        >
          <Send aria-hidden="true" className="h-4 w-4" />
          {submitMutation.isPending ? '提交中…' : '提交评估'}
        </Button>
        {presentation ? (
          <FormAlert
            title={presentation.title}
            description={presentation.message}
            code={isApiProblemError(submitError) ? submitError.code : undefined}
            requestId={isApiProblemError(submitError) ? submitError.requestId : undefined}
          />
        ) : null}
      </section>

      <section
        aria-label="总结历史"
        className="space-y-3 rounded-lg border border-border bg-surface p-4"
      >
        <h2 className="text-base font-medium text-text">总结历史</h2>
        {historyQuery.isPending ? (
          <p className="text-sm text-text-muted">正在读取总结历史…</p>
        ) : null}
        {historyQuery.isError ? (
          <ErrorState
            title="总结历史加载失败"
            error={historyQuery.error}
            onRetry={() => void historyQuery.refetch()}
          />
        ) : null}
        {historyQuery.isSuccess && entries.length === 0 ? (
          <p className="text-sm text-text-muted">暂无已提交的总结。</p>
        ) : null}
        <ul className="divide-y divide-border">
          {entries.map((entry) => (
            <li key={entry.id} className="flex flex-wrap items-center gap-3 py-3">
              <div className="min-w-0 flex-1">
                <p className="line-clamp-2 whitespace-pre-wrap text-sm text-text">
                  {entry.summary_text || '学习总结'}
                </p>
                <p className="text-xs text-text-subtle">
                  {formatDateTime(entry.created_at)} ·{' '}
                  {
                    { queued: '排队中', evaluating: '评估中', succeeded: '已完成', failed: '失败' }[
                      entry.status
                    ]
                  }
                </p>
              </div>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  saveLastEvaluationId(lessonId, entry.id)
                  setEvaluationId(entry.id)
                }}
              >
                查看评估
              </Button>
              <DeleteContentButton
                kind="summary"
                id={entry.id}
                name={formatDateTime(entry.created_at)}
                lessonId={lessonId}
                onDeleted={() => {
                  if (evaluationId === entry.id) {
                    forgetLastEvaluationId(lessonId)
                    setEvaluationId(null)
                  }
                  queryClient.removeQueries({
                    queryKey: queryKeys.summaryEvaluation(entry.id),
                    exact: true,
                  })
                }}
              />
            </li>
          ))}
        </ul>
        {historyQuery.hasNextPage ? (
          <Button
            variant="secondary"
            size="sm"
            disabled={historyQuery.isFetchingNextPage}
            onClick={() => void historyQuery.fetchNextPage()}
          >
            {historyQuery.isFetchingNextPage ? '加载中…' : '加载更多总结'}
          </Button>
        ) : null}
      </section>

      <ResourceJobs query={jobsQuery} title="总结正在评估" />

      {evaluationId !== null ? (
        evaluationQuery.isPending ? (
          <div role="status" aria-busy="true" aria-label="评估结果加载中" className="space-y-3">
            <SkeletonText lines={2} />
            <SkeletonText lines={3} />
          </div>
        ) : evaluationQuery.isError ? (
          <ErrorState
            title="评估结果加载失败"
            error={evaluationQuery.error}
            onRetry={() => void evaluationQuery.refetch()}
          />
        ) : (
          <EvaluationResult evaluation={evaluationQuery.data} />
        )
      ) : entries.length > 0 ? (
        <p className="text-sm text-text-muted">从总结历史中选择一条查看评估。</p>
      ) : (
        <section
          aria-label="评估说明"
          className="flex items-start gap-3 rounded-md border border-border bg-surface p-4"
        >
          <FileText aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-text-subtle" />
          <p className="text-sm text-text-muted">
            还没有提交过总结。提交后这里会显示 AI 的覆盖度评分、误解与遗漏分析。
          </p>
        </section>
      )}
    </div>
  )
}

/** 评估结果视图：进行中 / 失败 / 成功三种状态严格区分。 */
function EvaluationResult({ evaluation }: { evaluation: SummaryEvaluation }) {
  if (evaluation.status === 'queued' || evaluation.status === 'evaluating') {
    return (
      <section aria-label="评估进行中" className="rounded-md border border-border bg-surface p-4">
        <p className="text-sm text-text">AI 评估进行中…</p>
        <p className="mt-1 text-sm text-text-muted">
          可以离开本页；评估完成后会从任务中心看到结果，本页也会自动更新。
        </p>
      </section>
    )
  }

  if (evaluation.status === 'failed') {
    return (
      <section
        aria-label="评估失败"
        className="space-y-2 rounded-md border border-danger/40 bg-danger-bg p-4"
      >
        <p className="text-sm font-medium text-danger">AI 评估失败</p>
        <p className="text-sm text-text-muted">
          {evaluation.error?.detail ??
            '评估任务未能完成。你的总结已保留在输入框中，可修改后重新提交。'}
        </p>
        {evaluation.error?.code ? (
          <p className="text-xs text-text-subtle">错误码：{evaluation.error.code}</p>
        ) : null}
      </section>
    )
  }

  return (
    <section aria-label="评估结果" className="space-y-4">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-md border border-border bg-surface p-4">
        <Badge variant="ok">评估完成</Badge>
        {typeof evaluation.coverage_score === 'number' ? (
          <p className="text-sm text-text">
            要点覆盖度{' '}
            <strong className="text-base">{Math.round(evaluation.coverage_score * 100)}%</strong>
          </p>
        ) : null}
        <p className="text-sm text-text-subtle">提交于 {formatDateTime(evaluation.created_at)}</p>
      </div>

      <FeedbackList title="已掌握的要点" items={evaluation.correct_points} variant="ok" />
      <FeedbackList title="存在的误解" items={evaluation.misconceptions} variant="danger" />
      <FeedbackList title="遗漏的要点" items={evaluation.missing_points} variant="warn" />
      <FeedbackList title="学习建议" items={evaluation.suggestions} variant="info" />
    </section>
  )
}

function FeedbackList({
  title,
  items,
  variant,
}: {
  title: string
  items: string[]
  variant: 'ok' | 'danger' | 'warn' | 'info'
}) {
  return (
    <div className="rounded-md border border-border bg-surface p-4">
      <div className="flex items-center gap-2">
        <Badge variant={variant}>{title}</Badge>
        <span className="text-xs text-text-subtle">{items.length} 条</span>
      </div>
      {items.length > 0 ? (
        <ul className="mt-2 list-disc space-y-1 pl-5 text-sm leading-6 text-text">
          {items.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      ) : (
        <p className="mt-2 text-sm text-text-muted">无</p>
      )}
    </div>
  )
}
