import { describe, expect, it } from 'vitest'

import {
  getResourceBusyInfo,
  presentJobError,
  presentProblem,
  presentProblemCode,
} from '@/api/problem-messages'
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

  it('claude_code_settings_unavailable 指向运行服务 HOME 与设置权限', () => {
    const message = presentProblemCode('claude_code_settings_unavailable').message
    expect(message).toContain('运行服务 HOME')
    expect(message).toContain('~/.claude')
    expect(message).toContain('权限')
    expect(message).not.toMatch(/本机|宿主机|挂载/)
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

describe('presentJobError', () => {
  // 链接 + Key + prompt 片段：任何呈现文案都不得回显这类上游 detail。
  const URL_SENTINEL = 'https://www.youtube.com/watch?v=secret0&utm_source=sk-ant-leaked'

  it('URL 执行的稳定 Job code 都有可行动文案，且不含链接/Key/prompt 泄漏', () => {
    const codes = [
      'url_blocked',
      'material_fetch_failed',
      'material_empty_content',
      'material_too_large',
      'claude_timeout',
      'claude_invalid_request',
      'claude_invalid_json',
      'claude_schema_invalid',
      'claude_not_configured',
    ]
    for (const code of codes) {
      for (const retryable of [true, false]) {
        const presentation = presentJobError(code, retryable)
        const text = `${presentation.title} ${presentation.message}`
        expect(presentation.title.length, code).toBeGreaterThan(0)
        expect(presentation.message.length, code).toBeGreaterThan(0)
        expect(text, code).not.toContain(URL_SENTINEL)
        expect(text, code).not.toContain('sk-ant')
        expect(text, code).not.toContain('utm_source')
        // 保留 Job 的 retryable 信号
        expect(presentation.retryable, code).toBe(retryable)
      }
    }
  })

  it('material_fetch_failed 只靠 retryable 区分暂时性失败与内容不可访问', () => {
    const temporary = presentJobError('material_fetch_failed', true)
    const permanent = presentJobError('material_fetch_failed', false)
    expect(temporary.message).not.toBe(permanent.message)
    expect(temporary.retryable).toBe(true)
    expect(permanent.retryable).toBe(false)
    expect(temporary.message).toContain('重试')
    expect(permanent.message).toContain('无法')
  })

  it('empty/oversize 说明下一步操作', () => {
    expect(presentJobError('material_empty_content', false).message).toContain('文本')
    expect(presentJobError('material_too_large', false).message).toMatch(/文本|文件/)
  })

  it('claude_invalid_request 指引检查模型、Base URL 与 Anthropic 兼容性', () => {
    const message = presentJobError('claude_invalid_request', false).message
    expect(message).toContain('模型')
    expect(message).toContain('Base URL')
    expect(message).toContain('Anthropic')
    expect(message).not.toContain('Web Fetch')
  })

  it('未映射的 Job code 回落到契约呈现（未知码兜底 unknown_error）', () => {
    expect(presentJobError('claude_unavailable', true)).toEqual({
      ...presentProblemCode('claude_unavailable'),
      retryable: true,
    })
    expect(presentJobError('some_future_job_code', false)).toEqual({
      ...presentProblemCode('unknown_error'),
      retryable: false,
    })
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
