import { useEffect, useState } from 'react'

import type { Attempt, Question } from '@/api/generated/model'
import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { FormAlert } from '@/components/ui/form-alert'
import { Skeleton, SkeletonText } from '@/components/ui/skeleton'
import { MarkdownReader } from '@/features/documents/markdown/markdown-reader'
import { cn } from '@/lib/cn'

import { AnswerConflictDialog } from './answer-conflict-dialog'
import { hasAnswerValue } from './answer-value'
import { QuestionInput } from './question-input'
import { QuestionSyncChip, SyncStatusBar } from './sync-status-bar'
import { useAnswerSync } from './use-answer-sync'
import { useSubmitAttempt } from './use-submit-attempt'

const QUESTION_TYPE_LABEL: Record<Question['type'], string> = {
  single_choice: '单选',
  multiple_choice: '多选',
  true_false: '判断',
  short_answer: '简答',
  case_analysis: '案例分析',
}

/**
 * 作答专注视图（in_progress Attempt）。
 * - 输入即写内存 + sessionStorage，800ms debounce 自动保存；
 *   失焦/切题/页面隐藏/提交前 flush（useEffect 内注册）。
 * - 提交前必须全部同步；version 取引擎已见最大值；成功后锁定并切到结果视图。
 */
export function AttemptTaking({ attempt }: { attempt: Attempt }) {
  const engine = useAnswerSync(attempt)

  if (!engine) {
    return (
      <div role="status" aria-busy="true" aria-label="作答初始化中" className="space-y-4">
        <SkeletonText lines={2} />
        <Skeleton className="h-32 w-full" />
      </div>
    )
  }
  return <AttemptTakingInner attempt={attempt} engine={engine} />
}

function AttemptTakingInner({
  attempt,
  engine,
}: {
  attempt: Attempt
  engine: NonNullable<ReturnType<typeof useAnswerSync>>
}) {
  const submitMutation = useSubmitAttempt(attempt.id)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [submitError, setSubmitError] = useState<unknown>(null)
  const [conflictOpen, setConflictOpen] = useState(false)

  // flush 触发点：页面隐藏与离开（pagehide 覆盖关闭标签页/刷新）。
  // 草稿在 sessionStorage，网络 flush 是 best-effort，恢复由下次加载继续。
  useEffect(() => {
    const flush = () => {
      void engine.flushAll()
    }
    const onVisibilityChange = () => {
      if (document.visibilityState === 'hidden') flush()
    }
    document.addEventListener('visibilitychange', onVisibilityChange)
    window.addEventListener('pagehide', flush)
    return () => {
      document.removeEventListener('visibilitychange', onVisibilityChange)
      window.removeEventListener('pagehide', flush)
    }
  }, [engine])

  const questions = attempt.questions
  const answeredCount = questions.filter((question) =>
    hasAnswerValue(question, engine.getValue(question.id)),
  ).length
  const unansweredCount = questions.length - answeredCount
  const locking = submitMutation.isPending

  const handleSubmit = async () => {
    setSubmitError(null)
    const synced = await engine.flushAll()
    if (!synced) {
      setSubmitError(new Error('仍有答案未同步或存在冲突，请处理后再提交。'))
      return
    }
    submitMutation.mutate(
      { version: engine.getAttemptVersion() },
      {
        onSuccess: () => {
          engine.discardDraft()
        },
        onError: (error) => setSubmitError(error),
      },
    )
  }

  return (
    <div className="space-y-5">
      <SyncStatusBar engine={engine} onOpenConflict={() => setConflictOpen(true)} />
      <AnswerConflictDialog key={String(conflictOpen)} engine={engine} questions={questions} />

      {/* 题号导航：锚点跳转 + 已答标记；点击即 flush（切题语义） */}
      <nav aria-label="题目导航" className="rounded-md border border-border bg-surface p-3">
        <p className="text-xs text-text-subtle">
          已答 {answeredCount} / {questions.length}
        </p>
        <ol className="mt-2 flex flex-wrap gap-1.5">
          {questions.map((question, index) => {
            const answered = hasAnswerValue(question, engine.getValue(question.id))
            return (
              <li key={question.id}>
                <a
                  href={`#question-${question.id}`}
                  onClick={() => engine.flushAll()}
                  aria-label={`第 ${index + 1} 题，${answered ? '已答' : '未答'}`}
                  className={cn(
                    'flex h-8 w-8 items-center justify-center rounded-md border text-sm',
                    answered
                      ? 'border-accent bg-accent-bg text-accent-strong'
                      : 'border-border text-text-muted',
                  )}
                >
                  {index + 1}
                </a>
              </li>
            )
          })}
        </ol>
      </nav>

      <ol className="space-y-4">
        {questions.map((question, index) => (
          <li key={question.id} id={`question-${question.id}`} className="scroll-mt-16">
            <fieldset
              disabled={locking}
              className="rounded-lg border border-border bg-surface p-4"
              aria-busy={engine.getStatus(question.id) === 'saving'}
            >
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <legend className="sr-only">
                  第 {index + 1} 题，{QUESTION_TYPE_LABEL[question.type]}，{question.score} 分
                </legend>
                <p aria-hidden="true" className="text-xs text-text-subtle">
                  第 {index + 1} 题 · {QUESTION_TYPE_LABEL[question.type]} · {question.score} 分
                </p>
                <QuestionSyncChip
                  engine={engine}
                  questionId={question.id}
                  onOpenConflict={() => setConflictOpen(true)}
                />
              </div>
              <div className="mt-2 text-sm leading-6 text-text">
                <MarkdownReader markdown={question.stem} />
              </div>
              <div className="mt-3">
                <QuestionInput
                  question={question}
                  value={engine.getValue(question.id)}
                  disabled={locking || engine.getStatus(question.id) === 'conflict'}
                  onChange={(value) => engine.setLocal(question.id, value)}
                  onBlur={() => engine.flushQuestion(question.id)}
                />
              </div>
            </fieldset>
          </li>
        ))}
      </ol>

      {submitError ? (
        <FormAlert
          title={
            isApiProblemError(submitError)
              ? presentProblem(submitError).title
              : (submitError as Error).message
          }
          description={
            isApiProblemError(submitError) ? presentProblem(submitError).message : undefined
          }
          code={isApiProblemError(submitError) ? submitError.code : undefined}
          requestId={isApiProblemError(submitError) ? submitError.requestId : undefined}
        />
      ) : null}

      <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border bg-surface p-4">
        <p className="text-sm text-text-muted">
          {unansweredCount > 0 ? `还有 ${unansweredCount} 题未作答。` : '全部题目已作答。'}
          提交后不能再修改答案。
        </p>
        <Button
          variant="primary"
          onClick={() => setConfirmOpen(true)}
          disabled={locking || engine.getOverallStatus() === 'conflict'}
        >
          {locking ? '提交中…' : '提交作答'}
        </Button>
      </div>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="提交作答？"
        description={
          unansweredCount > 0
            ? `已答 ${answeredCount} / ${questions.length}，${unansweredCount} 题未作答将按 0 分计。提交后答案锁定。`
            : '全部题目已作答。提交后答案锁定，等待批阅结果。'
        }
        confirmLabel="确认提交"
        busy={submitMutation.isPending}
        onConfirm={() => {
          setConfirmOpen(false)
          void handleSubmit()
        }}
      />
    </div>
  )
}
