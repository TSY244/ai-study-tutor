import { forwardRef, type HTMLAttributes, type MouseEvent } from 'react'
import { CircleAlert } from 'lucide-react'

import { cn } from '@/lib/cn'

export interface FormAlertItem {
  /** 关联输入的 DOM id；存在时条目渲染为链接，点击/回车聚焦该输入。 */
  fieldId?: string
  message: string
}

export interface FormAlertProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  title: string
  /** 非字段级错误说明（Problem code 映射的稳定文案）。 */
  description?: string
  /** 字段级错误列表（validation_error 汇总）。 */
  items?: FormAlertItem[]
  /** Problem code 诊断。 */
  code?: string
  /** 响应 X-Request-ID，供排查使用。 */
  requestId?: string
}

/**
 * 表单顶部错误汇总（02-frontend-handoff.md §7）：
 * role="alert" 即时播报；出现后可接收程序化焦点（tabIndex=-1），
 * 让键盘与屏幕阅读器用户在错误发生后有明确焦点归属。
 */
export const FormAlert = forwardRef<HTMLDivElement, FormAlertProps>(function FormAlert(
  { title, description, items, code, requestId, className, ...rest },
  ref,
) {
  const focusField = (event: MouseEvent<HTMLAnchorElement>, fieldId: string) => {
    event.preventDefault()
    document.getElementById(fieldId)?.focus()
  }

  return (
    <div
      ref={ref}
      role="alert"
      tabIndex={-1}
      className={cn('rounded-md border border-danger/40 bg-danger-bg p-4 text-text', className)}
      {...rest}
    >
      <p className="flex items-center gap-2 text-sm font-medium">
        <CircleAlert aria-hidden="true" className="h-4 w-4 shrink-0 text-danger" />
        {title}
      </p>
      {description ? <p className="mt-1 text-sm text-text-muted">{description}</p> : null}
      {items && items.length > 0 ? (
        <ul className="mt-2 list-disc space-y-1 pl-5 text-sm">
          {items.map((item) => (
            <li key={`${item.fieldId ?? 'form'}-${item.message}`}>
              {item.fieldId ? (
                <a
                  href={`#${item.fieldId}`}
                  className="text-danger underline underline-offset-2"
                  onClick={(event) => focusField(event, item.fieldId as string)}
                >
                  {item.message}
                </a>
              ) : (
                item.message
              )}
            </li>
          ))}
        </ul>
      ) : null}
      {code || requestId ? (
        <p className="mt-2 text-xs text-text-subtle">
          {code ? (
            <>
              错误码 <span className="font-mono">{code}</span>
            </>
          ) : null}
          {code && requestId ? '；' : null}
          {requestId ? (
            <>
              请求 ID <span className="font-mono break-all">{requestId}</span>
            </>
          ) : null}
        </p>
      ) : null}
    </div>
  )
})
