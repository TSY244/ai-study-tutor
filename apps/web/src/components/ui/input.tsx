import { forwardRef, type InputHTMLAttributes } from 'react'

import { cn } from '@/lib/cn'

/**
 * 文本输入：aria-invalid 时红边框；焦点环来自全局 :focus-visible token。
 * 与 React Hook Form 的 register 兼容（forwardRef）。
 */
export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  function Input({ className, ...rest }, ref) {
    return (
      <input
        ref={ref}
        className={cn(
          'h-9 w-full rounded-md border border-border-strong bg-surface px-3 text-sm text-text',
          'placeholder:text-text-subtle disabled:cursor-not-allowed disabled:opacity-60',
          'aria-[invalid=true]:border-danger',
          className,
        )}
        {...rest}
      />
    )
  },
)
