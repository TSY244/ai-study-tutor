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
 * orval 生成 Client 的自定义 fetch 实例（mutator，orval v8 契约：
 * `(url, options?: RequestInit) => Promise<T>`，T 为解析后的响应 body）。
 *
 * 职责边界：
 * - 只使用同源相对路径（生产由 Caddy 反代，开发由 Vite proxy 转发）。
 * - 每个请求生成 X-Request-ID；调用方可经 options.headers 透传
 *   Idempotency-Key / X-Request-ID，以调用方值为准。
 * - 以服务端响应 X-Request-ID 为最终诊断 ID（客户端值无效时服务端会换新）。
 * - 非 2xx 统一抛 ApiProblemError（Problem Details），绝不返回裸 Response。
 * - 204 无响应体返回 undefined。
 * - FormData 不设置 Content-Type（fetch 自动生成 boundary）。
 */

function resolveUrl(path: string): string {
  // 浏览器：同源绝对 URL（Node/测试环境下相对路径无法直接 fetch，
  // 测试由 MSW 按路径通配拦截，主机名不参与匹配）。
  const base = globalThis.location?.origin ?? 'http://127.0.0.1:8080'
  return new URL(path, base).toString()
}

function createRequestId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `req-${Date.now()}-${Math.random()}`
}

/** 从 URL 中提取同源相对路径（含查询串），用于诊断记录。 */
function toRelativePath(url: string): string {
  try {
    const parsed = new URL(url, globalThis.location?.origin ?? 'http://127.0.0.1:8080')
    return `${parsed.pathname}${parsed.search}`
  } catch {
    return url
  }
}

export async function apiFetch<T>(url: string, options: RequestInit = {}): Promise<T> {
  const method = (options.method ?? 'GET').toUpperCase()
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  if (!headers.has('X-Request-ID')) headers.set('X-Request-ID', createRequestId())
  const requestId = headers.get('X-Request-ID') as string
  if (options.body instanceof FormData) headers.delete('Content-Type')

  const relativePath = toRelativePath(url)
  const startedAt = performance.now()
  const record = (status: number, code?: string, id = requestId) =>
    recordDiagnostic({
      requestId: id,
      method,
      url: relativePath,
      status,
      code,
      durationMs: Math.round(performance.now() - startedAt),
      at: new Date().toISOString(),
    })

  let response: Response
  try {
    response = await fetch(resolveUrl(url), { ...options, method, headers })
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === 'AbortError') throw cause
    const error = problemFromNetworkError(requestId, cause)
    record(0, error.code)
    throw error
  }

  // 服务端始终返回 X-Request-ID；若客户端值无效服务端会生成新值，以响应为准。
  const responseRequestId = response.headers.get('X-Request-ID') ?? requestId

  if (response.status === 204) {
    record(204, undefined, responseRequestId)
    return undefined as T
  }

  const contentType = response.headers.get('Content-Type')

  if (!response.ok) {
    const retryAfterSeconds = parseRetryAfter(response.headers.get('Retry-After'))
    if (isProblemContentType(contentType)) {
      const problemBody: unknown = await response.json().catch(() => undefined)
      const error = problemFromBody(problemBody, {
        status: response.status,
        requestId: responseRequestId,
        retryAfterSeconds,
      })
      record(error.status, error.code, responseRequestId)
      throw error
    }
    const error = problemFromHttpError(response.status, response.statusText, responseRequestId)
    record(error.status, error.code, responseRequestId)
    throw error
  }

  record(response.status, undefined, responseRequestId)

  if (contentType?.toLowerCase().includes('application/json')) {
    return (await response.json()) as T
  }
  // 非 JSON 成功响应（如 SSE、文件下载）不经过生成 Client 的 JSON 路径；
  // 此处按文本返回以保持可诊断性，专用 adapter（SSE 等）在后续阶段实现。
  return (await response.text()) as T
}

export { ApiProblemError }
