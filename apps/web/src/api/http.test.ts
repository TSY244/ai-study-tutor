import { http, HttpResponse } from 'msw'
import { afterEach, describe, expect, it } from 'vitest'

import { clearDiagnostics, getDiagnostics } from '@/api/diagnostics'
import { apiFetch } from '@/api/http'
import { ApiProblemError } from '@/api/problem'
import { server } from '@/mocks/server'
import { problemResponse } from '@/test/mock-helpers'

afterEach(() => clearDiagnostics())

describe('apiFetch 成功路径', () => {
  it('GET 返回解析后的 JSON body，并记录诊断', async () => {
    server.use(
      http.get('*/api/v1/probe', ({ request }) =>
        HttpResponse.json(
          { ok: true },
          { headers: { 'X-Request-ID': request.headers.get('X-Request-ID') ?? '' } },
        ),
      ),
    )

    const data = await apiFetch<{ ok: boolean }>('/api/v1/probe', { method: 'GET' })
    expect(data).toEqual({ ok: true })

    const [entry] = getDiagnostics()
    expect(entry.method).toBe('GET')
    expect(entry.url).toBe('/api/v1/probe')
    expect(entry.status).toBe(200)
    expect(entry.requestId).toBeTruthy()
  })

  it('每个请求自动生成 X-Request-ID（UUID）', async () => {
    let seen: string | null = null
    server.use(
      http.get('*/api/v1/probe', ({ request }) => {
        seen = request.headers.get('X-Request-ID')
        return HttpResponse.json({})
      }),
    )
    await apiFetch('/api/v1/probe', { method: 'GET' })
    expect(seen).toMatch(/^[0-9a-f-]{36}$/)
  })

  it('调用方透传的 X-Request-ID 优先；响应头覆盖为最终诊断 ID', async () => {
    server.use(
      http.get('*/api/v1/probe', () =>
        HttpResponse.json({}, { headers: { 'X-Request-ID': 'server-assigned' } }),
      ),
    )
    await apiFetch('/api/v1/probe', {
      method: 'GET',
      headers: { 'X-Request-ID': 'client-fixed' },
    })
    const [entry] = getDiagnostics()
    expect(entry.requestId).toBe('server-assigned')
  })

  it('204 返回 undefined', async () => {
    server.use(http.delete('*/api/v1/probe', () => new HttpResponse(null, { status: 204 })))
    await expect(apiFetch('/api/v1/probe', { method: 'DELETE' })).resolves.toBeUndefined()
  })
})

describe('apiFetch 错误路径', () => {
  it('422 problem+json -> ApiProblemError（code/字段错误/requestId）', async () => {
    server.use(
      http.post('*/api/v1/courses', () =>
        problemResponse({
          status: 422,
          code: 'validation_error',
          errors: [{ field: 'title', code: 'required', message: '课程名称不能为空' }],
        }),
      ),
    )

    const error = await apiFetch('/api/v1/courses', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: '' }),
    }).catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiProblemError)
    const problem = error as ApiProblemError
    expect(problem.code).toBe('validation_error')
    expect(problem.status).toBe(422)
    expect(problem.fieldErrors[0]?.field).toBe('title')
    expect(problem.requestId).toBe('req-test-1')

    const [entry] = getDiagnostics()
    expect(entry.code).toBe('validation_error')
    expect(entry.status).toBe(422)
  })

  it('409 + Retry-After -> retryAfterSeconds', async () => {
    server.use(
      http.post('*/api/v1/probe', () =>
        problemResponse(
          { status: 409, code: 'idempotency_in_progress' },
          { headers: { 'Retry-After': '2' } },
        ),
      ),
    )
    const error = await apiFetch('/api/v1/probe', { method: 'POST' }).catch((e: unknown) => e)
    expect((error as ApiProblemError).retryAfterSeconds).toBe(2)
  })

  it('非 Problem 的 500（HTML 错误页）-> http_error', async () => {
    server.use(
      http.get(
        '*/api/v1/probe',
        () => new HttpResponse('<html>oops</html>', { status: 500, statusText: 'Server Error' }),
      ),
    )
    const error = await apiFetch('/api/v1/probe', { method: 'GET' }).catch((e: unknown) => e)
    expect((error as ApiProblemError).code).toBe('http_error')
    expect((error as ApiProblemError).status).toBe(500)
  })

  it('网络层失败 -> network_error（status 0）', async () => {
    server.use(http.get('*/api/v1/probe', () => HttpResponse.error()))
    const error = await apiFetch('/api/v1/probe', { method: 'GET' }).catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiProblemError)
    expect((error as ApiProblemError).code).toBe('network_error')
    expect((error as ApiProblemError).status).toBe(0)
  })
})
