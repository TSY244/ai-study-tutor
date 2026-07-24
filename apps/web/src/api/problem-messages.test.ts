import { describe, expect, it } from 'vitest'

import { getResourceBusyInfo, presentProblem, presentProblemCode } from '@/api/problem-messages'
import { ApiProblemError, KNOWN_PROBLEM_CODES } from '@/api/problem'

describe('presentProblemCode', () => {
  it('契约全部已知 code 都有稳定映射（标题/建议/retryable）', () => {
    for (const code of KNOWN_PROBLEM_CODES) {
      const presentation = presentProblemCode(code)
      expect(presentation.title.length, code).toBeGreaterThan(0)
      expect(presentation.message.length, code).toBeGreaterThan(0)
      expect(typeof presentation.retryable, code).toBe('boolean')
    }
  })

  it('F2 要求的十个 code 语义稳定', () => {
    expect(presentProblemCode('validation_error').retryable).toBe(false)
    expect(presentProblemCode('version_conflict').retryable).toBe(false)
    expect(presentProblemCode('resource_busy').retryable).toBe(false)
    expect(presentProblemCode('idempotency_in_progress').retryable).toBe(true)
    expect(presentProblemCode('claude_not_configured').retryable).toBe(false)
    expect(presentProblemCode('claude_unavailable').retryable).toBe(true)
    expect(presentProblemCode('job_not_retryable').retryable).toBe(false)
    expect(presentProblemCode('storage_full').retryable).toBe(false)
    expect(presentProblemCode('upload_too_large').retryable).toBe(false)
    expect(presentProblemCode('network_error').retryable).toBe(true)
  })

  it('未映射的 code 兜底为 unknown_error，不崩溃', () => {
    expect(presentProblemCode('some_future_code')).toEqual(presentProblemCode('unknown_error'))
  })

  it('claude_not_configured 文案不要求浏览器保存 Key', () => {
    expect(presentProblemCode('claude_not_configured').message).toContain('部署侧')
  })
})

describe('presentProblem', () => {
  it('Problem 错误按 code 分支（不看 detail）', () => {
    const error = new ApiProblemError({
      status: 503,
      code: 'claude_unavailable',
      title: 'ignored',
      detail: 'ignored natural language',
      requestId: 'req-1',
    })
    expect(presentProblem(error)).toEqual(presentProblemCode('claude_unavailable'))
  })

  it('非 Problem 错误按 unknown_error 处理', () => {
    expect(presentProblem(new Error('boom'))).toEqual(presentProblemCode('unknown_error'))
  })
})

describe('getResourceBusyInfo', () => {
  it('提取 active_job_id 与 generation_family 扩展', () => {
    const error = new ApiProblemError({
      status: 409,
      code: 'resource_busy',
      title: '冲突',
      detail: '已有任务',
      requestId: 'req-2',
      extensions: { active_job_id: 'job-123', generation_family: 'lesson_generation' },
    })
    expect(getResourceBusyInfo(error)).toEqual({
      activeJobId: 'job-123',
      generationFamily: 'lesson_generation',
    })
  })

  it('非 resource_busy 返回 null', () => {
    const error = new ApiProblemError({
      status: 400,
      code: 'validation_error',
      title: 'x',
      detail: 'x',
      requestId: 'req-3',
    })
    expect(getResourceBusyInfo(error)).toBeNull()
    expect(getResourceBusyInfo(new Error('boom'))).toBeNull()
  })
})
