import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'

import type { Lesson } from '@/api/generated/model'
import { isApiProblemError, isProblemCode } from '@/api/problem'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { urlImportSchema, type UrlImportValues } from '@/features/materials/import-schemas'
import { ImportErrorAlert } from '@/features/materials/import-error-alert'
import { useImportMaterial } from '@/features/materials/use-import-material'
import { bindServerFieldErrors } from '@/lib/server-field-errors'

const KNOWN_FIELDS = ['url', 'preferred_language'] as const

/** URL 导入：仅 https:// 链接；抓取与生成都发生在后台任务中。 */
export function UrlImportForm({ lesson }: { lesson: Lesson }) {
  const { importUrl } = useImportMaterial(lesson)
  const alertRef = useRef<HTMLDivElement>(null)
  const inFlightRef = useRef(false)
  const [submitError, setSubmitError] = useState<unknown>(null)

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<UrlImportValues>({
    resolver: zodResolver(urlImportSchema),
    defaultValues: { url: '', preferred_language: '' },
  })

  const pending = isSubmitting || importUrl.isPending

  useEffect(() => {
    if (submitError) alertRef.current?.focus()
  }, [submitError])

  const onSubmit = handleSubmit(async (values) => {
    if (inFlightRef.current) return
    inFlightRef.current = true
    setSubmitError(null)
    try {
      const preferredLanguage = values.preferred_language.trim()
      await importUrl.mutateAsync({
        url: values.url.trim(),
        ...(preferredLanguage ? { preferred_language: preferredLanguage } : {}),
      })
    } catch (error) {
      if (isProblemCode(error, 'validation_error') && isApiProblemError(error)) {
        bindServerFieldErrors(setError, error.fieldErrors, KNOWN_FIELDS)
      }
      setSubmitError(error)
    } finally {
      inFlightRef.current = false
    }
  })

  return (
    <form noValidate onSubmit={onSubmit} className="space-y-4">
      {submitError ? <ImportErrorAlert ref={alertRef} error={submitError} /> : null}

      <div className="space-y-1.5">
        <Label htmlFor="url-import-url" required>
          链接
        </Label>
        <p className="text-sm text-text-muted">
          由 Claude 在后台提取允许的 https:// 链接内容，并生成原文、教纲与笔记；离开页面任务仍继续。
        </p>
        <Input
          id="url-import-url"
          type="url"
          inputMode="url"
          autoComplete="off"
          required
          placeholder="https://example.com/article"
          aria-invalid={errors.url ? true : undefined}
          aria-describedby={errors.url ? 'url-import-url-error' : undefined}
          {...register('url')}
        />
        {errors.url ? (
          <p id="url-import-url-error" className="text-sm text-danger">
            {errors.url.message}
          </p>
        ) : null}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="url-import-language">语言偏好</Label>
        <Input
          id="url-import-language"
          type="text"
          autoComplete="off"
          placeholder="可选，如 zh、en"
          aria-invalid={errors.preferred_language ? true : undefined}
          aria-describedby={errors.preferred_language ? 'url-import-language-error' : undefined}
          {...register('preferred_language')}
        />
        {errors.preferred_language ? (
          <p id="url-import-language-error" className="text-sm text-danger">
            {errors.preferred_language.message}
          </p>
        ) : null}
      </div>

      <Button type="submit" size="sm" disabled={pending}>
        {pending ? '提交中…' : '导入链接'}
      </Button>
    </form>
  )
}
