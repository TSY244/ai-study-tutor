import type { FieldError, Problem } from '@/api/generated/model'

/**
 * 后端错误统一为 application/problem+json（RFC 9457 Problem Details）。
 * 前端按稳定 code 分支，绝不解析自然语言 detail。
 */

/** 契约中的稳定错误码（04-api-contract.md §3）；另加前端本地合成码。 */
export const KNOWN_PROBLEM_CODES = [
  'bad_request',
  'not_found',
  'version_conflict',
  'idempotency_conflict',
  'idempotency_in_progress',
  'resource_busy',
  'attempt_already_submitted',
  'validation_error',
  'practice_not_needed',
  'upload_too_large',
  'unsupported_media_type',
  'too_many_requests',
  'claude_unavailable',
  'claude_not_configured',
  'maintenance_mode',
  'storage_full',
  // 文档规定（02 §7 / 08）但发布契约尚未枚举：POST /jobs/{id}/retry 的 409。
  // 前端按文档要求预置；待后端在 api/openapi.yaml 补充后即为正式契约码。
  'job_not_retryable',
  // 前端本地合成：网络层失败（无响应）与非 Problem 的 HTTP 错误
  'network_error',
  'http_error',
  'unknown_error',
] as const

export type KnownProblemCode = (typeof KNOWN_PROBLEM_CODES)[number]

export class ApiProblemError extends Error {
  override readonly name = 'ApiProblemError'
  readonly status: number
  readonly code: string
  readonly title: string
  readonly detail: string
  readonly type?: string
  readonly instance?: string
  /** 响应 X-Request-ID（服务端始终返回）；网络错误时为客户端生成的请求 ID。 */
  readonly requestId: string
  /** validation_error 的字段级错误，用于绑定到表单输入。 */
  readonly fieldErrors: FieldError[]
  /** Problem 扩展字段：active_job_id、current_answer、current_answer_version、
   *  current_attempt_version、generation_family 等，原样保留。 */
  readonly extensions: Record<string, unknown>
  /** idempotency_in_progress / 429 时服务端给出的 Retry-After（秒）。 */
  readonly retryAfterSeconds?: number

  constructor(init: {
    status: number
    code: string
    title: string
    detail: string
    type?: string
    instance?: string
    requestId: string
    fieldErrors?: FieldError[]
    extensions?: Record<string, unknown>
    retryAfterSeconds?: number
  }) {
    super(`${init.code}: ${init.detail}`)
    this.status = init.status
    this.code = init.code
    this.title = init.title
    this.detail = init.detail
    this.type = init.type
    this.instance = init.instance
    this.requestId = init.requestId
    this.fieldErrors = init.fieldErrors ?? []
    this.extensions = init.extensions ?? {}
    this.retryAfterSeconds = init.retryAfterSeconds
  }

  getExtension<T = unknown>(key: string): T | undefined {
    return this.extensions[key] as T | undefined
  }
}

export function isApiProblemError(error: unknown): error is ApiProblemError {
  return error instanceof ApiProblemError
}

export function isProblemCode(error: unknown, code: KnownProblemCode): boolean {
  return isApiProblemError(error) && error.code === code
}

export function isProblemContentType(contentType: string | null): boolean {
  return contentType?.toLowerCase().includes('application/problem+json') ?? false
}

const PROBLEM_BASE_KEYS = new Set([
  'type',
  'title',
  'status',
  'detail',
  'instance',
  'code',
  'request_id',
  'errors',
])

/** 把服务端 Problem body 解析为 ApiProblemError；body 结构不完整时保守降级。 */
export function problemFromBody(
  body: unknown,
  init: { status: number; requestId: string; retryAfterSeconds?: number },
): ApiProblemError {
  const p = (body ?? {}) as Partial<Problem> & Record<string, unknown>
  const extensions: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(p)) {
    if (!PROBLEM_BASE_KEYS.has(key)) extensions[key] = value
  }
  return new ApiProblemError({
    status: typeof p.status === 'number' ? p.status : init.status,
    code: typeof p.code === 'string' ? p.code : 'unknown_error',
    title: typeof p.title === 'string' ? p.title : '请求失败',
    detail: typeof p.detail === 'string' ? p.detail : '服务端返回了无法解析的错误。',
    type: typeof p.type === 'string' ? p.type : undefined,
    instance: typeof p.instance === 'string' ? p.instance : undefined,
    requestId: typeof p.request_id === 'string' ? p.request_id : init.requestId,
    fieldErrors: Array.isArray(p.errors) ? (p.errors as FieldError[]) : [],
    extensions,
    retryAfterSeconds: init.retryAfterSeconds,
  })
}

/** 非 Problem 的 HTTP 错误（如代理层 502、HTML 错误页）合成为 Problem。 */
export function problemFromHttpError(
  status: number,
  statusText: string,
  requestId: string,
): ApiProblemError {
  return new ApiProblemError({
    status,
    code: 'http_error',
    title: 'HTTP 错误',
    detail: `请求失败（${status} ${statusText || '未知状态'}）。`,
    requestId,
  })
}

/** fetch 拒绝（断网、连接拒绝、DNS）时合成网络错误；保留用户输入由 UI 层保证。 */
export function problemFromNetworkError(requestId: string, cause: unknown): ApiProblemError {
  const causeMessage = cause instanceof Error ? cause.message : String(cause)
  return new ApiProblemError({
    status: 0,
    code: 'network_error',
    title: '网络错误',
    detail: `无法连接服务，请确认本地服务已启动后重试。（${causeMessage}）`,
    requestId,
  })
}

export function parseRetryAfter(value: string | null): number | undefined {
  if (!value) return undefined
  const seconds = Number(value)
  return Number.isFinite(seconds) && seconds >= 0 ? seconds : undefined
}
