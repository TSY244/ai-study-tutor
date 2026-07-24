import { getUploadMaterialUrl } from '@/api/generated/endpoints/materials/materials'
import type { MaterialJobAccepted, UploadMaterialBody } from '@/api/generated/model'
import { recordDiagnostic } from '@/api/diagnostics'
import {
  ApiProblemError,
  isProblemContentType,
  parseRetryAfter,
  problemFromBody,
  problemFromHttpError,
  problemFromNetworkError,
} from '@/api/problem'

/**
 * 文件上传专用 adapter：生成的 Client 走 fetch，而 fetch 没有成熟的上传
 * 进度事件，因此这里用 XMLHttpRequest 实现独立上传进度（02 §6.1：上传
 * 进度与后台 Job 进度必须区分）。契约不变——URL 仍取自生成为
 * getUploadMaterialUrl 的同一端点，错误同样归一为 ApiProblemError。
 */

export interface UploadProgress {
  loaded: number
  /** lengthComputable=false 时为 undefined（此时只展示已传字节）。 */
  total?: number
  percent?: number
}

export interface UploadOptions {
  idempotencyKey: string
  onProgress?: (progress: UploadProgress) => void
  signal?: AbortSignal
}

export function uploadMaterialWithProgress(
  lessonId: string,
  body: UploadMaterialBody,
  options: UploadOptions,
): Promise<MaterialJobAccepted> {
  const formData = new FormData()
  formData.append('file', body.file)
  if (body.preferred_language !== undefined) {
    formData.append('preferred_language', body.preferred_language)
  }
  if (body.transcription_mode !== undefined) {
    formData.append('transcription_mode', body.transcription_mode)
  }

  const requestId = globalThis.crypto.randomUUID()
  const startedAt = performance.now()
  const record = (status: number, code?: string, id: string = requestId) =>
    recordDiagnostic({
      requestId: id,
      method: 'POST',
      url: getUploadMaterialUrl(lessonId),
      status,
      code,
      durationMs: Math.round(performance.now() - startedAt),
      at: new Date().toISOString(),
    })

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    // 同源相对路径：生产由 Caddy 反代，开发由 Vite proxy 转发。
    xhr.open('POST', getUploadMaterialUrl(lessonId))
    xhr.setRequestHeader('Accept', 'application/json')
    xhr.setRequestHeader('X-Request-ID', requestId)
    xhr.setRequestHeader('Idempotency-Key', options.idempotencyKey)

    options.signal?.addEventListener('abort', () => xhr.abort())

    xhr.upload.addEventListener('progress', (event) => {
      options.onProgress?.({
        loaded: event.loaded,
        total: event.lengthComputable ? event.total : undefined,
        percent: event.lengthComputable
          ? Math.round((event.loaded / event.total) * 100)
          : undefined,
      })
    })

    xhr.addEventListener('load', () => {
      const responseRequestId = xhr.getResponseHeader('X-Request-ID') ?? requestId
      const contentType = xhr.getResponseHeader('Content-Type')
      if (xhr.status >= 200 && xhr.status < 300) {
        record(xhr.status, undefined, responseRequestId)
        try {
          resolve(JSON.parse(xhr.responseText) as MaterialJobAccepted)
        } catch (cause) {
          const error = problemFromNetworkError(responseRequestId, cause)
          record(0, error.code, responseRequestId)
          reject(error)
        }
        return
      }
      const retryAfterSeconds = parseRetryAfter(xhr.getResponseHeader('Retry-After'))
      let error: ApiProblemError
      if (isProblemContentType(contentType)) {
        try {
          error = problemFromBody(JSON.parse(xhr.responseText), {
            status: xhr.status,
            requestId: responseRequestId,
            retryAfterSeconds,
          })
        } catch {
          error = problemFromHttpError(xhr.status, xhr.statusText, responseRequestId)
        }
      } else {
        error = problemFromHttpError(xhr.status, xhr.statusText, responseRequestId)
      }
      record(error.status, error.code, responseRequestId)
      reject(error)
    })

    xhr.addEventListener('error', () => {
      const error = problemFromNetworkError(requestId, new Error('网络请求失败'))
      record(0, error.code)
      reject(error)
    })
    xhr.addEventListener('abort', () => {
      reject(new DOMException('上传已取消', 'AbortError'))
    })

    xhr.send(formData)
  })
}
