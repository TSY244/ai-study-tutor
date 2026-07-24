import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'

import type { Lesson } from '@/api/generated/model'
import { isApiProblemError, isProblemCode } from '@/api/problem'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { textImportSchema, type TextImportValues } from '@/features/materials/import-schemas'
import { ImportErrorAlert } from '@/features/materials/import-error-alert'
import { useImportMaterial } from '@/features/materials/use-import-material'
import { bindServerFieldErrors } from '@/lib/server-field-errors'

const KNOWN_FIELDS = ['title', 'text'] as const

/** 文本导入：粘贴正文 → 202 后由课时面板切换为任务进度（上传与任务分离）。 */
export function TextImportForm({ lesson }: { lesson: Lesson }) {
  const { importText } = useImportMaterial(lesson)
  const alertRef = useRef<HTMLDivElement>(null)
  const inFlightRef = useRef(false)
  const [submitError, setSubmitError] = useState<unknown>(null)

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<TextImportValues>({
    resolver: zodResolver(textImportSchema),
    defaultValues: { title: '', text: '' },
  })

  const pending = isSubmitting || importText.isPending

  useEffect(() => {
    if (submitError) alertRef.current?.focus()
  }, [submitError])

  const onSubmit = handleSubmit(async (values) => {
    if (inFlightRef.current) return
    inFlightRef.current = true
    setSubmitError(null)
    try {
      const title = values.title.trim()
      await importText.mutateAsync({
        ...(title ? { title } : {}),
        text: values.text,
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
        <Label htmlFor="text-import-title">素材标题</Label>
        <Input
          id="text-import-title"
          type="text"
          autoComplete="off"
          placeholder="可选，默认“文本素材”"
          aria-invalid={errors.title ? true : undefined}
          aria-describedby={errors.title ? 'text-import-title-error' : undefined}
          {...register('title')}
        />
        {errors.title ? (
          <p id="text-import-title-error" className="text-sm text-danger">
            {errors.title.message}
          </p>
        ) : null}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="text-import-text" required>
          文本内容
        </Label>
        <Textarea
          id="text-import-text"
          rows={8}
          required
          placeholder="粘贴讲义、教材章节或转写文本。"
          aria-invalid={errors.text ? true : undefined}
          aria-describedby={errors.text ? 'text-import-text-error' : undefined}
          {...register('text')}
        />
        {errors.text ? (
          <p id="text-import-text-error" className="text-sm text-danger">
            {errors.text.message}
          </p>
        ) : null}
      </div>

      <Button type="submit" size="sm" disabled={pending}>
        {pending ? '提交中…' : '导入文本'}
      </Button>
    </form>
  )
}
