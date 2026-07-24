import { z } from 'zod'

import type { CourseCreate } from '@/api/generated/model'

/**
 * 课程表单校验：约束与契约 CourseCreate 一致（api/openapi.yaml）。
 * 边界：DTO 类型以生成 model 为准，这里只声明校验规则，不重复定义实体。
 */
export const courseFormSchema = z.object({
  title: z
    .string()
    .min(1, '请输入课程标题。')
    .max(200, '课程标题最长 200 字。')
    .refine((value) => value.trim().length > 0, '请输入课程标题。'),
  description: z.string().max(5000, '课程描述最长 5000 字。'),
})

export type CourseFormValues = z.infer<typeof courseFormSchema>

/** 表单值 -> 契约载荷：去首尾空白；空描述省略该字段。 */
export function toCourseCreate(values: CourseFormValues): CourseCreate {
  const description = values.description.trim()
  return {
    title: values.title.trim(),
    ...(description ? { description } : {}),
  }
}
