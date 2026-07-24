import { CircleAlert } from 'lucide-react'

import { isApiProblemError } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

export interface ErrorStateProps {
  error: unknown
  onRetry?: () => void
  title?: string
  className?: string
}

/**
 * 查询失败框架：Problem code -> 稳定文案（不看自然语言 detail）
 * + 错误码 + Request ID 诊断。role="alert" 让屏幕阅读器即时播报。
 */
export function ErrorState({ error, onRetry, title, className }: ErrorStateProps) {
  const problem = isApiProblemError(error) ? error : null
  const presentation = presentProblem(error)
  return (
    <div
      role="alert"
      className={cn('rounded-md border border-border-strong bg-surface p-4', className)}
    >
      <p className="flex items-center gap-2 font-medium text-text">
        <CircleAlert aria-hidden="true" className="h-4 w-4 shrink-0 text-danger" />
        {title ?? presentation.title}
      </p>
      <p className="mt-1 text-sm text-text-muted">{presentation.message}</p>
      {problem ? (
        <p className="mt-2 text-sm text-text-subtle">
          <span className="font-medium">错误码</span>{' '}
          <span className="font-mono">{problem.code}</span>
          {problem.requestId ? (
            <>
              ；请求 ID <span className="font-mono break-all">{problem.requestId}</span>
            </>
          ) : null}
        </p>
      ) : null}
      {onRetry ? (
        <Button variant="secondary" size="sm" className="mt-4" onClick={onRetry}>
          重试
        </Button>
      ) : null}
    </div>
  )
}
