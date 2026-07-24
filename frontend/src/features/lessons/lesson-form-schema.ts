import { z } from 'zod'

import type { LessonCreate } from '@/api/generated/model'

/**
 * 课时表单校验：约束与契约 LessonCreate 一致（api/openapi.yaml）。
 * DTO 类型以生成 model 为准，这里只声明校验规则。
 */
export const lessonFormSchema = z.object({
  ordinal: z
    .number({ error: '请输入课次（≥1 的整数）。' })
    .int('课次必须是整数。')
    .min(1, '课次从 1 开始。'),
  title: z
    .string()
    .min(1, '请输入课时标题。')
    .max(200, '课时标题最长 200 字。')
    .refine((value) => value.trim().length > 0, '请输入课时标题。'),
  description: z.string().max(5000, '课时描述最长 5000 字。'),
})

export type LessonFormValues = z.infer<typeof lessonFormSchema>

/** 表单值 -> 契约载荷：去首尾空白；空描述省略该字段。 */
export function toLessonCreate(values: LessonFormValues): LessonCreate {
  const description = values.description.trim()
  return {
    ordinal: values.ordinal,
    title: values.title.trim(),
    ...(description ? { description } : {}),
  }
}
