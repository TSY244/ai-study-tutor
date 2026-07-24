import { useMutation, useQueryClient } from '@tanstack/react-query'

import {
  createTextMaterial,
  createUrlMaterial,
} from '@/api/generated/endpoints/materials/materials'
import type {
  Lesson,
  MaterialJobAccepted,
  TextMaterialCreate,
  UploadMaterialBody,
  UrlMaterialCreate,
} from '@/api/generated/model'
import { invalidateMaterialImportAccepted } from '@/api/invalidation'
import { isApiProblemError } from '@/api/problem'
import { uploadMaterialWithProgress, type UploadProgress } from '@/api/upload'
import {
  beginIdempotencyIntent,
  resolveIdempotencyIntent,
  rotateIdempotencyIntent,
} from '@/lib/idempotency'

/**
 * 素材导入意图 scope（每课时一个）。同一课时同时只允许一个活动生成任务，
 * 因此文本/URL/上传共享同一意图：任一导入进行中都复用同一 Key。
 */
export function importMaterialIntentScope(lessonId: string): string {
  return `import-material:${lessonId}`
}

export interface UploadInput {
  body: UploadMaterialBody
  onProgress?: (progress: UploadProgress) => void
}

/**
 * 三种来源（文本/URL/文件）共用的导入 Mutation 集合（02 §6.1 同一创建语义）：
 * - 202 接受：结束意图，精确失效课时/课时列表/任务中心，UI 转入任务进度；
 * - 409 resource_busy：保留同一 Key（绝不换 Key 再建任务），由 UI 关联
 *   active_job_id 展示现有任务；
 * - validation_error / idempotency_conflict：载荷必变，轮换 Key；
 * - network_error / 5xx：保留同一 Key 供重试。
 */
export function useImportMaterial(lesson: Lesson) {
  const queryClient = useQueryClient()
  const scope = importMaterialIntentScope(lesson.id)

  const onSuccess = async (accepted: MaterialJobAccepted) => {
    resolveIdempotencyIntent(scope)
    await invalidateMaterialImportAccepted(
      queryClient,
      lesson.id,
      lesson.course_id,
      accepted.job.id,
    )
  }
  const onError = (error: unknown) => {
    if (
      isApiProblemError(error) &&
      (error.code === 'validation_error' || error.code === 'idempotency_conflict')
    ) {
      rotateIdempotencyIntent(scope)
    }
  }

  const importText = useMutation({
    mutationFn: (input: TextMaterialCreate) =>
      createTextMaterial(lesson.id, input, {
        headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) },
      }),
    onSuccess,
    onError,
  })

  const importUrl = useMutation({
    mutationFn: (input: UrlMaterialCreate) =>
      createUrlMaterial(lesson.id, input, {
        headers: { 'Idempotency-Key': beginIdempotencyIntent(scope) },
      }),
    onSuccess,
    onError,
  })

  const importUpload = useMutation({
    mutationFn: (input: UploadInput) =>
      uploadMaterialWithProgress(lesson.id, input.body, {
        idempotencyKey: beginIdempotencyIntent(scope),
        onProgress: input.onProgress,
      }),
    onSuccess,
    onError,
  })

  return { importText, importUrl, importUpload }
}
