import { forwardRef, type LabelHTMLAttributes } from 'react'

import { cn } from '@/lib/cn'

export interface LabelProps extends LabelHTMLAttributes<HTMLLabelElement> {
  /** 必填标记：视觉 *（aria-hidden），语义由输入的 required/aria-required 表达。 */
  required?: boolean
}

export const Label = forwardRef<HTMLLabelElement, LabelProps>(function Label(
  { className, required = false, children, ...rest },
  ref,
) {
  return (
    <label ref={ref} className={cn('block text-sm font-medium text-text', className)} {...rest}>
      {children}
      {required ? (
        <span aria-hidden="true" className="ml-0.5 text-danger">
          *
        </span>
      ) : null}
    </label>
  )
})
