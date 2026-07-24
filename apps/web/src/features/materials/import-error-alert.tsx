import { forwardRef } from 'react'
import { Link } from '@tanstack/react-router'

import { isApiProblemError } from '@/api/problem'
import { getResourceBusyInfo, presentProblem } from '@/api/problem-messages'
import { FormAlert, type FormAlertItem } from '@/components/ui/form-alert'

/**
 * 导入表单的错误汇总：在通用 Problem 映射之上，resource_busy 额外关联
 * 现有任务（active_job_id）——不新建任务、不换 Key，引导用户去看
 * 正在进行的 lesson_generation 任务。
 */
export const ImportErrorAlert = forwardRef<
  HTMLDivElement,
  { error: unknown; items?: FormAlertItem[] }
>(function ImportErrorAlert({ error, items }, ref) {
  const presentation = presentProblem(error)
  const busy = getResourceBusyInfo(error)
  return (
    <div className="space-y-2">
      <FormAlert
        ref={ref}
        title={presentation.title}
        description={presentation.message}
        items={items}
        code={isApiProblemError(error) ? error.code : undefined}
        requestId={isApiProblemError(error) ? error.requestId : undefined}
      />
      {busy?.activeJobId ? (
        <p className="text-sm">
          <Link
            to="/jobs/$jobId"
            params={{ jobId: busy.activeJobId }}
            className="rounded-sm font-medium text-accent underline underline-offset-2"
          >
            查看进行中的生成任务
          </Link>
          <span className="ml-2 text-text-subtle">
            （{busy.generationFamily ?? 'lesson_generation'}）
          </span>
        </p>
      ) : null}
    </div>
  )
})
