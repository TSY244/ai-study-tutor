import { describe, expect, it } from 'vitest'

import { AttemptStatus, JobKind, JobStatus, LessonStatus, QuizStatus } from '@/api/generated/model'
import {
  ATTEMPT_STATUS_META,
  isJobActive,
  JOB_KIND_LABEL,
  JOB_STATUS_META,
  LESSON_STATUS_META,
  QUIZ_STATUS_META,
  systemStatusMeta,
} from '@/lib/status'

/**
 * 状态矩阵与契约枚举的同步测试：契约新增状态时这里立刻失败，
 * 强制同步更新文案，杜绝“未映射状态”流入 UI。
 */
describe('状态矩阵', () => {
  it('JobStatus 全覆盖', () => {
    expect(Object.keys(JOB_STATUS_META).sort()).toEqual(Object.keys(JobStatus).sort())
  })

  it('JobKind 全覆盖', () => {
    expect(Object.keys(JOB_KIND_LABEL).sort()).toEqual(Object.keys(JobKind).sort())
  })

  it('LessonStatus / AttemptStatus / QuizStatus 全覆盖', () => {
    expect(Object.keys(LESSON_STATUS_META).sort()).toEqual(Object.keys(LessonStatus).sort())
    expect(Object.keys(ATTEMPT_STATUS_META).sort()).toEqual(Object.keys(AttemptStatus).sort())
    expect(Object.keys(QUIZ_STATUS_META).sort()).toEqual(Object.keys(QuizStatus).sort())
  })

  it('每个状态都有非空文案与已知 Badge 变体', () => {
    const variants = new Set(['neutral', 'ok', 'warn', 'danger', 'info', 'accent'])
    for (const meta of Object.values(JOB_STATUS_META)) {
      expect(meta.label.length).toBeGreaterThan(0)
      expect(variants.has(meta.variant)).toBe(true)
    }
  })

  it('isJobActive 只把进行中状态算作活跃', () => {
    for (const status of ['queued', 'running', 'validating', 'cancel_requested'] as const) {
      expect(isJobActive(status)).toBe(true)
    }
    for (const status of ['succeeded', 'failed', 'cancelled'] as const) {
      expect(isJobActive(status)).toBe(false)
    }
  })

  it('系统状态未知值回退为原始文本 + neutral', () => {
    expect(systemStatusMeta('ok')).toEqual({ label: '正常', variant: 'ok' })
    expect(systemStatusMeta('whatever')).toEqual({ label: 'whatever', variant: 'neutral' })
  })
})
