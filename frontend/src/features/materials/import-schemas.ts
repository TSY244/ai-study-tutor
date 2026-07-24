import { z } from 'zod'

/** 文本/URL 导入校验：约束与契约 TextMaterialCreate / UrlMaterialCreate 一致。 */
export const textImportSchema = z.object({
  title: z.string().max(200, '素材标题最长 200 字。'),
  text: z
    .string()
    .min(1, '请输入要导入的文本。')
    .refine((value) => value.trim().length > 0, '请输入要导入的文本。'),
})
export type TextImportValues = z.infer<typeof textImportSchema>

export const urlImportSchema = z.object({
  url: z
    .string()
    .min(1, '请输入链接。')
    .regex(/^https:\/\//, '只支持 https:// 开头的链接。'),
  preferred_language: z.string().max(30, '语言偏好最长 30 字。'),
})
export type UrlImportValues = z.infer<typeof urlImportSchema>
