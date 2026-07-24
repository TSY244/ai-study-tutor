import { describe, expect, it } from 'vitest'

import {
  ApiProblemError,
  isProblemCode,
  parseRetryAfter,
  problemFromBody,
  problemFromHttpError,
  problemFromNetworkError,
} from '@/api/problem'

describe('problemFromBody', () => {
  it('解析完整 Problem Details，含字段错误与扩展字段', () => {
    const error = problemFromBody(
      {
        type: 'https://ai-study-tutor.local/problems/validation-error',
        title: '请求参数无效',
        status: 422,
        detail: '请修正标记的字段后重试。',
        code: 'validation_error',
        request_id: 'srv-1',
        errors: [{ field: 'title', code: 'required', message: '课程名称不能为空' }],
      },
      { status: 422, requestId: 'client-1' },
    )

    expect(error).toBeInstanceOf(ApiProblemError)
    expect(error.code).toBe('validation_error')
    expect(error.status).toBe(422)
    expect(error.requestId).toBe('srv-1') // 服务端 request_id 优先
    expect(error.fieldErrors).toHaveLength(1)
    expect(error.fieldErrors[0]?.field).toBe('title')
  })

  it('保留扩展字段（resource_busy 的 active_job_id / generation_family）', () => {
    const error = problemFromBody(
      {
        type: 'https://ai-study-tutor.local/problems/resource-busy',
        title: '资源忙',
        status: 409,
        detail: '该课时有正在运行的生成任务。',
        code: 'resource_busy',
        request_id: 'srv-2',
        active_job_id: 'job-abc',
        generation_family: 'lesson_generation',
      },
      { status: 409, requestId: 'client-1' },
    )

    expect(error.getExtension('active_job_id')).toBe('job-abc')
    expect(error.getExtension('generation_family')).toBe('lesson_generation')
    // 基础字段不进入扩展
    expect(error.getExtension('code')).toBeUndefined()
    expect(error.getExtension('request_id')).toBeUndefined()
  })

  it('保留 Retry-After 秒数', () => {
    const error = problemFromBody(
      { status: 409, code: 'idempotency_in_progress', request_id: 'srv-3' },
      { status: 409, requestId: 'client-1', retryAfterSeconds: 2 },
    )
    expect(error.retryAfterSeconds).toBe(2)
  })

  it('body 缺失或畸形时保守降级，不抛异常', () => {
    const error = problemFromBody(undefined, { status: 500, requestId: 'client-9' })
    expect(error.code).toBe('unknown_error')
    expect(error.status).toBe(500)
    expect(error.requestId).toBe('client-9')
  })
})

describe('problemFromHttpError / problemFromNetworkError', () => {
  it('非 Problem 的 HTTP 错误合成为 http_error', () => {
    const error = problemFromHttpError(502, 'Bad Gateway', 'req-x')
    expect(error.code).toBe('http_error')
    expect(error.status).toBe(502)
    expect(error.requestId).toBe('req-x')
  })

  it('网络层失败合成为 network_error', () => {
    const error = problemFromNetworkError('req-y', new Error('connection refused'))
    expect(error.code).toBe('network_error')
    expect(error.status).toBe(0)
    expect(error.detail).toContain('connection refused')
  })
})

describe('isProblemCode', () => {
  it('按稳定 code 分支', () => {
    const error = problemFromHttpError(409, 'Conflict', 'req-z')
    expect(isProblemCode(error, 'http_error')).toBe(true)
    expect(isProblemCode(error, 'version_conflict')).toBe(false)
    expect(isProblemCode(new Error('x'), 'http_error')).toBe(false)
  })
})

describe('parseRetryAfter', () => {
  it.each([
    ['3', 3],
    ['0', 0],
    [null, undefined],
    ['abc', undefined],
    ['-1', undefined],
  ])('parseRetryAfter(%s) === %s', (input, expected) => {
    expect(parseRetryAfter(input)).toBe(expected)
  })
})
