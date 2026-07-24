import { forwardRef, type TextareaHTMLAttributes } from 'react'

import { cn } from '@/lib/cn'

/** 多行输入：与 Input 同一套边框/失效样式；允许垂直调整高度。 */
export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className, rows = 4, ...rest }, ref) {
  return (
    <textarea
      ref={ref}
      rows={rows}
      className={cn(
        'w-full resize-y rounded-md border border-border-strong bg-surface px-3 py-2 text-sm text-text',
        'placeholder:text-text-subtle disabled:cursor-not-allowed disabled:opacity-60',
        'aria-[invalid=true]:border-danger',
        className,
      )}
      {...rest}
    />
  )
})
