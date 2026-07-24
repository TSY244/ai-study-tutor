import { zodResolver } from '@hookform/resolvers/zod'
import { Link, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'

import { isApiProblemError, isProblemCode } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { buttonClasses } from '@/components/ui/button-styles'
import { FormAlert, type FormAlertItem } from '@/components/ui/form-alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  lessonFormSchema,
  toLessonCreate,
  type LessonFormValues,
} from '@/features/lessons/lesson-form-schema'
import { useCreateLesson } from '@/features/lessons/use-lesson-mutations'
import { bindServerFieldErrors } from '@/lib/server-field-errors'

const FIELD_IDS = {
  ordinal: 'lesson-ordinal',
  title: 'lesson-title',
  description: 'lesson-description',
} as const
const KNOWN_FIELDS = ['ordinal', 'title', 'description'] as const

interface ServerAlert {
  title: string
  description?: string
  items?: FormAlertItem[]
  code?: string
  requestId?: string
}

/**
 * 新建课时表单：创建空课时（draft）后进入课时页导入素材。
 * 模式与新建课程一致：RHF+Zod、服务端错误绑定、防重复提交、
 * Idempotency-Key 生命周期在 useCreateLesson 内。
 */
export function NewLessonForm({
  courseId,
  nextOrdinal,
}: {
  courseId: string
  nextOrdinal: number
}) {
  const navigate = useNavigate()
  const createLesson = useCreateLesson(courseId)
  const alertRef = useRef<HTMLDivElement>(null)
  const inFlightRef = useRef(false)
  const [serverAlert, setServerAlert] = useState<ServerAlert | null>(null)

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<LessonFormValues>({
    resolver: zodResolver(lessonFormSchema),
    defaultValues: { ordinal: nextOrdinal, title: '', description: '' },
  })

  const pending = isSubmitting || createLesson.isPending

  useEffect(() => {
    if (serverAlert) alertRef.current?.focus()
  }, [serverAlert])

  const onSubmit = handleSubmit(async (values) => {
    if (inFlightRef.current) return
    inFlightRef.current = true
    setServerAlert(null)
    try {
      const lesson = await createLesson.mutateAsync(toLessonCreate(values))
      await navigate({
        to: '/courses/$courseId/lessons/$lessonId',
        params: { courseId, lessonId: lesson.id },
      })
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
    <form noValidate onSubmit={onSubmit} className="max-w-xl space-y-5">
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
          placeholder="可选：这节课覆盖什么内容。"
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

      <div className="flex items-center gap-3">
        <Button type="submit" disabled={pending}>
          {pending ? '创建中…' : '创建课时'}
        </Button>
        <Link
          to="/courses/$courseId"
          params={{ courseId }}
          className={buttonClasses({ variant: 'secondary' })}
        >
          取消
        </Link>
      </div>
    </form>
  )
}
