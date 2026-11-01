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
  courseFormSchema,
  toCourseCreate,
  type CourseFormValues,
} from '@/features/courses/course-form-schema'
import { useCreateCourse } from '@/features/courses/use-create-course'
import { bindServerFieldErrors } from '@/lib/server-field-errors'

const FIELD_IDS = { title: 'course-title', description: 'course-description' } as const
const KNOWN_FIELDS = ['title', 'description'] as const

interface ServerAlert {
  title: string
  description?: string
  items?: FormAlertItem[]
  code?: string
  requestId?: string
}

/**
 * 新建课程表单（F2 首个真实表单，确立后续表单的统一模式）：
 * - React Hook Form + Zod 客户端校验；错误绑定到输入下方；
 * - 服务端 validation_error 绑定到字段 + 顶部汇总，未知字段不丢失；
 * - 重复提交保护：提交中按钮禁用，回车路径由 isPending 显式拦截，
 *   Idempotency-Key 在 hook 内兜底（双击/重试/刷新恢复同一 Key）；
 * - 任何错误都不清空输入；错误发生后焦点归属顶部 role=alert 汇总。
 */
export function NewCourseForm() {
  const navigate = useNavigate()
  const createCourse = useCreateCourse()
  const alertRef = useRef<HTMLDivElement>(null)
  // ref 级防重：同一事件循环内的双击/回车不依赖渲染后的 disabled 状态。
  const inFlightRef = useRef(false)
  const [serverAlert, setServerAlert] = useState<ServerAlert | null>(null)

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<CourseFormValues>({
    resolver: zodResolver(courseFormSchema),
    defaultValues: { title: '', description: '' },
  })

  const pending = isSubmitting || createCourse.isPending

  // 错误发生后焦点归属顶部汇总；role="alert" 同时负责屏幕阅读器播报。
  useEffect(() => {
    if (serverAlert) alertRef.current?.focus()
  }, [serverAlert])

  const onSubmit = handleSubmit(async (values) => {
    if (inFlightRef.current) return // 重复提交保护：双击/回车/连点只发一次
    inFlightRef.current = true
    setServerAlert(null)
    try {
      const course = await createCourse.mutateAsync(toCourseCreate(values))
      await navigate({ to: '/courses/$courseId', params: { courseId: course.id } })
    } catch (error) {
      // 输入保留：RHF 状态不重置，仅呈现错误。
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
        <Label htmlFor={FIELD_IDS.title}>课程标题（可选）</Label>
        <Input
          id={FIELD_IDS.title}
          type="text"
          autoComplete="off"
          placeholder="留空，后续由 AI 根据学习内容命名"
          aria-invalid={errors.title ? true : undefined}
          aria-describedby={
            errors.title
              ? `${FIELD_IDS.title}-hint ${FIELD_IDS.title}-error`
              : `${FIELD_IDS.title}-hint`
          }
          {...register('title')}
        />
        <p id={`${FIELD_IDS.title}-hint`} className="text-sm text-text-muted">
          留空时先显示“待 AI 命名”，首次生成学习总结后自动命名；手动填写的名称会保留。
        </p>
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
          placeholder="可选：这门课要学什么、给谁看。"
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

      <p className="text-sm text-text-muted">
        AI 总结统一包含：内容概览、核心知识点、知识结构、示例与应用、结论与复习、来源与局限。
      </p>

      <div className="flex items-center gap-3">
        <Button type="submit" disabled={pending}>
          {pending ? '创建中…' : '创建课程'}
        </Button>
        <Link to="/courses" className={buttonClasses({ variant: 'secondary' })}>
          取消
        </Link>
      </div>
    </form>
  )
}
