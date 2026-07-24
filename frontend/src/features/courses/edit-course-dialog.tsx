import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'

import type { Course } from '@/api/generated/model'
import { isApiProblemError, isProblemCode } from '@/api/problem'
import { presentProblem } from '@/api/problem-messages'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent } from '@/components/ui/dialog'
import { FormAlert, type FormAlertItem } from '@/components/ui/form-alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { courseFormSchema, type CourseFormValues } from '@/features/courses/course-form-schema'
import { useUpdateCourse } from '@/features/courses/use-course-mutations'
import { bindServerFieldErrors } from '@/lib/server-field-errors'

const FIELD_IDS = { title: 'edit-course-title', description: 'edit-course-description' } as const
const KNOWN_FIELDS = ['title', 'description'] as const

interface ServerAlert {
  title: string
  description?: string
  items?: FormAlertItem[]
  code?: string
  requestId?: string
}

export interface EditCourseDialogProps {
  course: Course
  open: boolean
  onOpenChange: (open: boolean) => void
}

/**
 * 编辑课程对话框：与新建课程同一校验 schema；PATCH 携带 version 乐观锁，
 * version_conflict 时按映射表提示刷新重试，不清空用户输入。
 */
export function EditCourseDialog({ course, open, onOpenChange }: EditCourseDialogProps) {
  const updateCourse = useUpdateCourse(course.id)
  const alertRef = useRef<HTMLDivElement>(null)
  const inFlightRef = useRef(false)
  const [serverAlert, setServerAlert] = useState<ServerAlert | null>(null)

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<CourseFormValues>({
    resolver: zodResolver(courseFormSchema),
    defaultValues: { title: course.title, description: course.description },
  })

  // 每次打开都重置为最新课程数据（其他操作可能已改了内容/版本）。
  useEffect(() => {
    if (open) {
      reset({ title: course.title, description: course.description })
      setServerAlert(null)
    }
  }, [open, course, reset])

  useEffect(() => {
    if (serverAlert) alertRef.current?.focus()
  }, [serverAlert])

  const pending = isSubmitting || updateCourse.isPending

  const onSubmit = handleSubmit(async (values) => {
    if (inFlightRef.current) return
    inFlightRef.current = true
    setServerAlert(null)
    try {
      const description = values.description.trim()
      await updateCourse.mutateAsync({
        title: values.title.trim(),
        description,
        version: course.version,
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
      <DialogContent title="编辑课程" description="修改课程标题与描述。">
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
            <Label htmlFor={FIELD_IDS.title} required>
              课程标题
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
            <Label htmlFor={FIELD_IDS.description}>课程描述</Label>
            <Textarea
              id={FIELD_IDS.description}
              rows={4}
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
