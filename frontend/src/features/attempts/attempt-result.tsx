import type { Attempt, Question } from '@/api/generated/model'
import { Badge } from '@/components/ui/badge'
import { MarkdownReader } from '@/features/documents/markdown/markdown-reader'
import { formatDateTime } from '@/lib/format'
import { ATTEMPT_STATUS_META } from '@/lib/status'

import { formatAnswerValue } from './answer-value'
import { GradingRetry } from './grading-retry'

/**
 * 作答结果视图（submitted / grading / graded / grading_failed）。
 * 提交前绝不渲染标准答案与解析——反馈字段只在提交后由服务端下发，
 * 本组件仅在非 in_progress 状态挂载。
 */
export function AttemptResult({ attempt }: { attempt: Attempt }) {
  return (
    <div className="space-y-5">
      <ScoreSummary attempt={attempt} />

      {attempt.status === 'grading' || attempt.status === 'submitted' ? (
        <section aria-label="批阅进行中" className="rounded-md border border-border bg-surface p-4">
          <p className="text-sm text-text">
            {attempt.status === 'grading' ? 'AI 批阅进行中…' : '已提交，等待批阅开始…'}
          </p>
          <p className="mt-1 text-sm text-text-muted">
            可以离开本页；批阅完成后会从任务中心看到结果，本页也会自动更新。
          </p>
        </section>
      ) : null}

      {attempt.status === 'grading_failed' ? (
        <section
          aria-label="批阅失败"
          className="space-y-3 rounded-md border border-danger/40 bg-danger-bg p-4"
        >
          <div>
            <p className="text-sm font-medium text-danger">AI 批阅失败</p>
            <p className="mt-1 text-sm text-text-muted">
              {attempt.grading_error_detail ?? '批阅任务未能完成。'}
              你的答案与客观题成绩已保留，可重新批阅。
            </p>
          </div>
          <GradingRetry attempt={attempt} />
        </section>
      ) : null}

      <section aria-label="逐题结果">
        <h2 className="text-sm font-medium text-text">逐题结果</h2>
        <ol className="mt-2 space-y-3">
          {attempt.questions.map((question, index) => (
            <QuestionResultItem
              key={question.id}
              attempt={attempt}
              question={question}
              index={index}
            />
          ))}
        </ol>
      </section>
    </div>
  )
}

function ScoreSummary({ attempt }: { attempt: Attempt }) {
  const meta = ATTEMPT_STATUS_META[attempt.status]
  return (
    <section aria-label="成绩概览" className="rounded-md border border-border bg-surface p-4">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
        <Badge variant={meta.variant}>{meta.label}</Badge>
        {typeof attempt.final_score === 'number' ? (
          <p className="text-sm text-text">
            最终成绩{' '}
            <strong className="text-base">
              {attempt.final_score}
              {typeof attempt.max_score === 'number' ? ` / ${attempt.max_score}` : ''}
            </strong>
          </p>
        ) : null}
        {typeof attempt.auto_score === 'number' ? (
          <p className="text-sm text-text-muted">
            客观题{attempt.status === 'graded' ? '得分' : '暂定分'} {attempt.auto_score}
            {typeof attempt.max_score === 'number' ? ` / ${attempt.max_score}` : ''}
          </p>
        ) : null}
        {attempt.status === 'grading' ? (
          <p className="text-sm text-text-muted">最终成绩等待 AI 批阅</p>
        ) : null}
        {attempt.submitted_at ? (
          <p className="text-sm text-text-subtle">提交于 {formatDateTime(attempt.submitted_at)}</p>
        ) : null}
      </div>
    </section>
  )
}

function QuestionResultItem({
  attempt,
  question,
  index,
}: {
  attempt: Attempt
  question: Question
  index: number
}) {
  const answer = attempt.answers.find((item) => item.question_id === question.id)
  const feedback = answer?.feedback ?? null

  return (
    <li className="rounded-lg border border-border bg-surface p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <p className="text-xs text-text-subtle">
          第 {index + 1} 题 · {question.score} 分
        </p>
        <div className="flex items-center gap-2">
          {answer?.is_correct === true ? <Badge variant="ok">正确</Badge> : null}
          {answer?.is_correct === false ? <Badge variant="danger">错误</Badge> : null}
          {answer && answer.is_correct === null && attempt.status !== 'graded' ? (
            <Badge variant="neutral">待批阅</Badge>
          ) : null}
          {typeof answer?.awarded_score === 'number' ? (
            <span className="text-sm text-text-muted">
              {answer.awarded_score} / {question.score} 分
            </span>
          ) : null}
        </div>
      </div>

      <div className="mt-2 text-sm leading-6 text-text">
        <MarkdownReader markdown={question.stem} />
      </div>

      <dl className="mt-3 space-y-2 text-sm">
        <div>
          <dt className="text-xs text-text-subtle">你的答案</dt>
          <dd className="mt-0.5 whitespace-pre-wrap break-words text-text">
            {answer ? formatAnswerValue(question, answer.answer.value) : '（未作答）'}
          </dd>
        </div>
        {feedback ? (
          <>
            {feedback.correct_answer !== null ? (
              <div>
                <dt className="text-xs text-text-subtle">正确答案</dt>
                <dd className="mt-0.5 text-text">
                  {formatAnswerValue(question, feedback.correct_answer.value)}
                </dd>
              </div>
            ) : null}
            <div>
              <dt className="text-xs text-text-subtle">反馈</dt>
              <dd className="mt-0.5 whitespace-pre-wrap break-words text-text">
                {feedback.feedback}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-text-subtle">正确理解</dt>
              <dd className="mt-0.5 whitespace-pre-wrap break-words text-text">
                {feedback.correct_understanding}
              </dd>
            </div>
            {feedback.explanation ? (
              <div>
                <dt className="text-xs text-text-subtle">解析</dt>
                <dd className="mt-0.5 whitespace-pre-wrap break-words text-text">
                  {feedback.explanation}
                </dd>
              </div>
            ) : null}
            {feedback.weak_concepts.length > 0 ? (
              <div>
                <dt className="text-xs text-text-subtle">薄弱知识点</dt>
                <dd className="mt-1 flex flex-wrap gap-1">
                  {feedback.weak_concepts.map((concept) => (
                    <Badge key={concept} variant="warn">
                      {concept}
                    </Badge>
                  ))}
                </dd>
              </div>
            ) : null}
          </>
        ) : null}
      </dl>
    </li>
  )
}
