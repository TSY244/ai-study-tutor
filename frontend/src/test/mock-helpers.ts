import { HttpResponse } from 'msw'

import type { Problem } from '@/api/generated/model'
import { server } from '@/mocks/server'

/** 构造 application/problem+json 响应（模拟中间件：总是带 X-Request-ID）。 */
export function problemResponse(
  overrides: Partial<Problem> & { status: number; code: string },
  init?: { headers?: HeadersInit },
): HttpResponse<Problem> {
  const { status } = overrides
  const body: Problem = {
    type: `https://ai-study-tutor.local/problems/${overrides.code}`,
    title: '测试错误',
    detail: '测试用 Problem Details。',
    request_id: 'req-test-1',
    ...overrides,
  }
  return HttpResponse.json(body, {
    status,
    headers: {
      'Content-Type': 'application/problem+json',
      'X-Request-ID': body.request_id,
      ...init?.headers,
    },
  })
}

export interface CapturedRequest {
  /** Idempotency-Key 请求头。 */
  key: string | null
  /** JSON 请求体（非 JSON 时为 null）。 */
  body: unknown
  /** 全部请求头（需要断言其他头时使用）。 */
  headers: Headers
}

/**
 * 用 MSW 生命周期事件旁路记录匹配请求（不替换任何 handler）。
 * 返回 requests 数组与退订函数 stop。
 */
export function captureRequests(
  method: string,
  pathname: string | RegExp,
): { requests: CapturedRequest[]; stop: () => void } {
  const requests: CapturedRequest[] = []
  const listener = ({ request }: { request: Request }) => {
    const path = new URL(request.url).pathname
    const matches = typeof pathname === 'string' ? path === pathname : pathname.test(path)
    if (request.method === method && matches) {
      const entry: CapturedRequest = {
        key: request.headers.get('Idempotency-Key'),
        body: null,
        headers: request.headers,
      }
      requests.push(entry)
      request
        .clone()
        .json()
        .then((body) => {
          entry.body = body
        })
        .catch(() => undefined)
    }
  }
  server.events.on('request:start', listener)
  return { requests, stop: () => server.events.removeListener('request:start', listener) }
}
