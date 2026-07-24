import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'

import type { Lesson } from '@/api/generated/model'
import { isApiProblemError, isProblemCode } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { FormAlert, type FormAlertItem } from '@/components/ui/form-alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { lessonFormSchema, type LessonFormValues } from '@/features/lessons/lesson-form-schema'
import { useUpdateLesson } from '@/features/lessons/use-lesson-mutations'
import { bindServerFieldErrors } from '@/lib/server-field-errors'

const FIELD_IDS = {
  ordinal: 'edit-lesson-ordinal',
  title: 'edit-lesson-title',
  description: 'edit-lesson-description',
} as const
const KNOWN_FIELDS = ['ordinal', 'title', 'description'] as const

interface ServerAlert {
  title: string
  description?: string
  items?: FormAlertItem[]
  code?: string
  requestId?: string
}

/** 编辑课时（课次/标题/描述）：PATCH 携带 version 乐观锁。 */
export function EditLessonDialog({
  lesson,
  open,
  onOpenChange,
}: {
  lesson: Lesson
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const updateLesson = useUpdateLesson(lesson.id, lesson.course_id)
  const alertRef = useRef<HTMLDivElement>(null)
  const inFlightRef = useRef(false)
  const [serverAlert, setServerAlert] = useState<ServerAlert | null>(null)

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<LessonFormValues>({
    resolver: zodResolver(lessonFormSchema),
    defaultValues: {
      ordinal: lesson.ordinal,
      title: lesson.title,
      description: lesson.description,
    },
  })

  useEffect(() => {
    if (open) {
      reset({
        ordinal: lesson.ordinal,
        title: lesson.title,
        description: lesson.description,
      })
      setServerAlert(null)
    }
  }, [open, lesson, reset])

  useEffect(() => {
    if (serverAlert) alertRef.current?.focus()
  }, [serverAlert])

  const pending = isSubmitting || updateLesson.isPending

  const onSubmit = handleSubmit(async (values) => {
    if (inFlightRef.current) return
    inFlightRef.current = true
    setServerAlert(null)
    try {
      const description = values.description.trim()
      await updateLesson.mutateAsync({
        ordinal: values.ordinal,
        title: values.title.trim(),
        description,
        version: lesson.version,
      })
      onOpenChange(false)
    } catch (error) {
      const presentation = presentProblem(error)
      if (isProblemCode(error, 'validation_error') && isApiProblemError(error)) {
        const unbound = bindServerFieldErrors(setError, error.fieldErrors, KNOWN_FIELDS)
        setServerAlert({
          ...presentation,
          items: unbound.map((fieldError) => ({
            message: `${fieldError.field}：${fieldError.message}`,
          })),
          code: error.code,
          requestId: error.requestId,
        })
      } else {
        setServerAlert({
          ...presentation,
          code: isApiProblemError(error) ? error.code : undefined,
          requestId: isApiProblemError(error) ? error.requestId : undefined,
        })
      }
    } finally {
      inFlightRef.current = false
    }
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent title="编辑课时" description="修改课次、标题与描述。">
        <form noValidate onSubmit={onSubmit} className="space-y-4">
          {serverAlert ? (
            <FormAlert
              ref={alertRef}
              title={serverAlert.title}
              description={serverAlert.description}
              items={serverAlert.items}
              code={serverAlert.code}
              requestId={serverAlert.requestId}
            />
          ) : null}

          <div className="space-y-1.5">
            <Label htmlFor={FIELD_IDS.ordinal} required>
              课次
            </Label>
            <Input
              id={FIELD_IDS.ordinal}
              type="number"
              min={1}
              step={1}
              required
              className="w-28"
              aria-invalid={errors.ordinal ? true : undefined}
              aria-describedby={errors.ordinal ? `${FIELD_IDS.ordinal}-error` : undefined}
              {...register('ordinal', { valueAsNumber: true })}
            />
            {errors.ordinal ? (
              <p id={`${FIELD_IDS.ordinal}-error`} className="text-sm text-danger">
                {errors.ordinal.message}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor={FIELD_IDS.title} required>
              课时标题
            </Label>
            <Input
              id={FIELD_IDS.title}
              type="text"
              autoComplete="off"
              required
              aria-invalid={errors.title ? true : undefined}
              aria-describedby={errors.title ? `${FIELD_IDS.title}-error` : undefined}
              {...register('title')}
            />
            {errors.title ? (
              <p id={`${FIELD_IDS.title}-error`} className="text-sm text-danger">
                {errors.title.message}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor={FIELD_IDS.description}>课时描述</Label>
            <Textarea
              id={FIELD_IDS.description}
              rows={3}
              aria-invalid={errors.description ? true : undefined}
              aria-describedby={errors.description ? `${FIELD_IDS.description}-error` : undefined}
              {...register('description')}
            />
            {errors.description ? (
              <p id={`${FIELD_IDS.description}-error`} className="text-sm text-danger">
                {errors.description.message}
              </p>
            ) : null}
          </div>

          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={pending}
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button type="submit" size="sm" disabled={pending}>
              {pending ? '保存中…' : '保存'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
