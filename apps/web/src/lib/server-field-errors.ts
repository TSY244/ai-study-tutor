import type { FieldValues, Path, UseFormSetError } from 'react-hook-form'

import type { FieldError } from '@/api/generated/model'

/**
 * validation_error 的字段错误绑定到对应输入（02-frontend-handoff.md §7）。
 * 返回无法匹配的字段错误——后端字段名漂移时仍展示在表单顶部汇总，
 * 不静默丢失。matched 字段由 RHF 在输入下方展示。
 */
export function bindServerFieldErrors<T extends FieldValues>(
  setError: UseFormSetError<T>,
  fieldErrors: readonly FieldError[],
  knownFields: readonly string[],
): FieldError[] {
  const known = new Set(knownFields)
  const unbound: FieldError[] = []
  for (const fieldError of fieldErrors) {
    if (known.has(fieldError.field)) {
      setError(fieldError.field as Path<T>, { type: 'server', message: fieldError.message })
    } else {
      unbound.push(fieldError)
    }
  }
  return unbound
}
