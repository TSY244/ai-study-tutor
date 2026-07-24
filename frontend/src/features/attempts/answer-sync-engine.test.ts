import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AnswerSave, SavedAnswer } from '@/api/generated/model'
import { ApiProblemError } from '@/api/problem'

import { AnswerSyncEngine } from './answer-sync-engine'
import { ATTEMPT_DRAFT_STORAGE_PREFIX, loadAttemptDraft } from './attempt-draft'

/**
 * 自动保存状态机单测（fake timers 控制 debounce）。
 * saveFn 为注入的模拟实现，可精确控制成功/409/网络失败与响应乱序。
 */

const ATTEMPT_ID = 'att-test-1'
const Q1 = 'q-1'
const Q2 = 'q-2'

function savedAnswer(
  questionId: string,
  answerVersion: number,
  attemptVersion: number,
): SavedAnswer {
  return {
    attempt_id: ATTEMPT_ID,
    question_id: questionId,
    answer: { value: 'B' },
    answer_version: answerVersion,
    attempt_version: attemptVersion,
    saved_at: '2026-07-19T00:00:00Z',
  }
}

interface Call {
  questionId: string
  body: AnswerSave
}

function createEngine(
  saveImpl: (questionId: string, body: AnswerSave) => Promise<SavedAnswer>,
  options?: {
    initialAnswers?: ConstructorParameters<typeof AnswerSyncEngine>[0]['initialAnswers']
    initialAttemptVersion?: number
  },
) {
  const calls: Call[] = []
  const saveFn = (questionId: string, body: AnswerSave) => {
    calls.push({ questionId, body: structuredClone(body) })
    return saveImpl(questionId, body)
  }
  const engine = new AnswerSyncEngine({
    attemptId: ATTEMPT_ID,
    questionIds: [Q1, Q2],
    initialAnswers: options?.initialAnswers ?? [],
    initialAttemptVersion: options?.initialAttemptVersion ?? 1,
    saveFn,
    debounceMs: 800,
    now: () => new Date('2026-07-19T10:00:00Z'),
  })
  return { engine, calls }
}

function conflictProblem(current: {
  value: unknown
  answerVersion: number
  attemptVersion: number
}): ApiProblemError {
  return new ApiProblemError({
    status: 409,
    code: 'version_conflict',
    title: '答案版本冲突',
    detail: '服务端已有更新的答案。',
    requestId: 'req-conflict',
    extensions: {
      current_answer: { value: current.value },
      current_answer_version: current.answerVersion,
      current_attempt_version: current.attemptVersion,
    },
  })
}

function networkProblem(): ApiProblemError {
  return new ApiProblemError({
    status: 0,
    code: 'network_error',
    title: '网络错误',
    detail: '无法连接服务。',
    requestId: 'req-net',
  })
}

beforeEach(() => {
  vi.useFakeTimers()
  sessionStorage.clear()
})

afterEach(() => {
  vi.useRealTimers()
  sessionStorage.clear()
})

describe('debounce 与版本语义', () => {
  it('连续输入合并为一次 PATCH（800ms），携带最新值与 answer_version=0', async () => {
    const { engine, calls } = createEngine((qid) => Promise.resolve(savedAnswer(qid, 1, 2)))

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(400)
    engine.setLocal(Q1, 'B')
    await vi.advanceTimersByTimeAsync(799)
    expect(calls).toHaveLength(0)

    await vi.advanceTimersByTimeAsync(1)
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0].body.answer).toEqual({ value: 'B' })
    expect(calls[0].body.answer_version).toBe(0)
    expect(calls[0].body.client_updated_at).toBe('2026-07-19T10:00:00.000Z')
    expect(engine.getStatus(Q1)).toBe('saved')
    expect(engine.getAttemptVersion()).toBe(2)
    engine.destroy()
  })

  it('首次保存成功后，第二次保存携带 answer_version=1', async () => {
    const { engine, calls } = createEngine((qid, body) =>
      Promise.resolve(savedAnswer(qid, body.answer_version + 1, 2 + body.answer_version)),
    )

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('saved'))

    engine.setLocal(Q1, 'C')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(calls).toHaveLength(2))
    expect(calls[1].body.answer_version).toBe(1)
    engine.destroy()
  })

  it('同题串行：保存进行中继续输入，完成后立即用新版本重发', async () => {
    let resolveFirst: ((value: SavedAnswer) => void) | undefined
    const { engine, calls } = createEngine((qid, body) => {
      if (body.answer_version === 0) {
        return new Promise<SavedAnswer>((resolve) => {
          resolveFirst = resolve
        })
      }
      return Promise.resolve(savedAnswer(qid, 2, 5))
    })

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(800)
    expect(calls).toHaveLength(1) // 第一次保存进行中

    engine.setLocal(Q1, 'B') // 保存期间输入
    resolveFirst?.(savedAnswer(Q1, 1, 4))
    await vi.waitFor(() => expect(calls).toHaveLength(2))
    expect(calls[1].body.answer).toEqual({ value: 'B' })
    expect(calls[1].body.answer_version).toBe(1)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('saved'))
    expect(engine.getAttemptVersion()).toBe(5)
    engine.destroy()
  })

  it('不同题并发：两题同时到期各自独立保存', async () => {
    const { engine, calls } = createEngine((qid) => Promise.resolve(savedAnswer(qid, 1, 3)))

    engine.setLocal(Q1, 'A')
    engine.setLocal(Q2, '短文答案')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(calls).toHaveLength(2))
    expect(calls.map((call) => call.questionId).sort()).toEqual([Q1, Q2])
    engine.destroy()
  })

  it('乱序响应：attempt_version 只增不减（取已见最大值）', async () => {
    const versions = [9, 3]
    const { engine } = createEngine((qid, body) =>
      Promise.resolve(savedAnswer(qid, body.answer_version + 1, versions[body.answer_version])),
    )

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('saved'))
    expect(engine.getAttemptVersion()).toBe(9)

    engine.setLocal(Q1, 'B')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('saved'))
    expect(engine.getAttemptVersion()).toBe(9) // 3 不覆盖 9
    engine.destroy()
  })
})

describe('失败与恢复', () => {
  it('网络失败标记 unsynced，dirty 保留；flushAll 重试成功', async () => {
    let fail = true
    const { engine, calls } = createEngine((qid) =>
      fail ? Promise.reject(networkProblem()) : Promise.resolve(savedAnswer(qid, 1, 2)),
    )

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('unsynced'))
    expect(engine.getOverallStatus()).toBe('unsynced')

    fail = false
    const ok = await engine.flushAll()
    expect(ok).toBe(true)
    expect(calls).toHaveLength(2)
    expect(engine.getStatus(Q1)).toBe('saved')
    expect(engine.getOverallStatus()).toBe('saved')
    engine.destroy()
  })

  it('409 且服务端值与本次发送值一致：视为先前保存成功，采用服务端版本', async () => {
    const { engine } = createEngine((qid, body) =>
      body.answer_version === 0
        ? Promise.reject(conflictProblem({ value: 'A', answerVersion: 3, attemptVersion: 7 }))
        : Promise.resolve(savedAnswer(qid, body.answer_version + 1, 8)),
    )

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('saved'))
    expect(engine.getAttemptVersion()).toBe(7)
    engine.destroy()
  })

  it('409 且值不同：进入 conflict，解决前不再自动保存', async () => {
    const { engine, calls } = createEngine(() =>
      Promise.reject(conflictProblem({ value: 'X', answerVersion: 2, attemptVersion: 6 })),
    )

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('conflict'))

    const conflict = engine.getConflict(Q1)
    expect(conflict).toMatchObject({ questionId: Q1, localValue: 'A', serverValue: 'X' })

    // 冲突未解决：debounce 不再触发新请求
    await vi.advanceTimersByTimeAsync(5000)
    expect(calls).toHaveLength(1)
    engine.destroy()
  })

  it('resolveConflict(local)：以服务端版本为基线重发本地值', async () => {
    let conflicted = true
    const { engine, calls } = createEngine((qid, body) => {
      if (conflicted) {
        conflicted = false
        return Promise.reject(conflictProblem({ value: 'X', answerVersion: 2, attemptVersion: 6 }))
      }
      return Promise.resolve(savedAnswer(qid, body.answer_version + 1, 7))
    })

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('conflict'))

    engine.resolveConflict(Q1, 'local')
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('saved'))
    expect(calls).toHaveLength(2)
    expect(calls[1].body.answer).toEqual({ value: 'A' })
    expect(calls[1].body.answer_version).toBe(2) // 服务端基线
    engine.destroy()
  })

  it('resolveConflict(server)：采用服务端值，本地修改被用户明确放弃', async () => {
    const { engine, calls } = createEngine(() =>
      Promise.reject(conflictProblem({ value: 'X', answerVersion: 2, attemptVersion: 6 })),
    )

    engine.setLocal(Q1, 'A')
    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('conflict'))

    engine.resolveConflict(Q1, 'server')
    expect(engine.getStatus(Q1)).toBe('saved')
    expect(engine.getValue(Q1)).toBe('X')
    expect(calls).toHaveLength(1)
    expect(engine.getAttemptVersion()).toBe(6)
    engine.destroy()
  })
})

describe('草稿持久化与刷新恢复', () => {
  it('输入立即写入 sessionStorage（未等 debounce）', () => {
    const { engine } = createEngine((qid) => Promise.resolve(savedAnswer(qid, 1, 2)))
    engine.setLocal(Q1, 'A')

    const draft = loadAttemptDraft(ATTEMPT_ID)
    expect(draft?.answers[Q1]).toMatchObject({ value: 'A', savedVersion: 0, dirty: true })
    expect(draft?.attemptVersion).toBe(1)
    engine.destroy()
  })

  it('刷新恢复：dirty 草稿覆盖显示值并以服务端版本为基线自动重发', async () => {
    sessionStorage.setItem(
      ATTEMPT_DRAFT_STORAGE_PREFIX + ATTEMPT_ID,
      JSON.stringify({
        version: 1,
        attemptId: ATTEMPT_ID,
        attemptVersion: 4,
        answers: {
          [Q1]: {
            value: 'B',
            savedVersion: 0,
            dirty: true,
            clientUpdatedAt: '2026-07-19T09:00:00Z',
          },
        },
        updatedAt: '2026-07-19T09:00:00Z',
      }),
    )

    const { engine, calls } = createEngine((qid, body) =>
      Promise.resolve(savedAnswer(qid, body.answer_version + 1, 5)),
    )
    expect(engine.getValue(Q1)).toBe('B')
    expect(engine.getStatus(Q1)).toBe('dirty')
    expect(engine.getAttemptVersion()).toBe(4) // max(GET=1, draft=4)

    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0].body.answer_version).toBe(0) // 服务端无此题答案，基线为 0
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('saved'))
    engine.destroy()
  })

  it('恢复时服务端状态比草稿旧（保存响应丢失）：采用草稿值并以服务端基线重发', async () => {
    // 上次会话已保存成功（draft savedVersion=1, dirty=false），但服务端应答丢失、
    // GET 返回无答案（answer_version 0）——草稿版本更新，应采用草稿值重发。
    sessionStorage.setItem(
      ATTEMPT_DRAFT_STORAGE_PREFIX + ATTEMPT_ID,
      JSON.stringify({
        version: 1,
        attemptId: ATTEMPT_ID,
        attemptVersion: 2,
        answers: {
          [Q1]: {
            value: 'B',
            savedVersion: 1,
            dirty: false,
            clientUpdatedAt: '2026-07-19T09:00:00Z',
          },
        },
        updatedAt: '2026-07-19T09:00:00Z',
      }),
    )

    const { engine, calls } = createEngine((qid, body) =>
      Promise.resolve(savedAnswer(qid, body.answer_version + 1, 3)),
    )
    expect(engine.getValue(Q1)).toBe('B')
    expect(engine.getStatus(Q1)).toBe('dirty')

    await vi.advanceTimersByTimeAsync(800)
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0].body.answer_version).toBe(0) // 服务端基线为 0，不是草稿的 1
    await vi.waitFor(() => expect(engine.getStatus(Q1)).toBe('saved'))
    engine.destroy()
  })

  it('恢复时服务端已保存相同值：直接 saved，不再重发', async () => {
    sessionStorage.setItem(
      ATTEMPT_DRAFT_STORAGE_PREFIX + ATTEMPT_ID,
      JSON.stringify({
        version: 1,
        attemptId: ATTEMPT_ID,
        attemptVersion: 3,
        answers: {
          [Q1]: {
            value: 'B',
            savedVersion: 0,
            dirty: true,
            clientUpdatedAt: '2026-07-19T09:00:00Z',
          },
        },
        updatedAt: '2026-07-19T09:00:00Z',
      }),
    )

    const { engine, calls } = createEngine((qid) => Promise.resolve(savedAnswer(qid, 2, 5)), {
      initialAnswers: [
        {
          question_id: Q1,
          answer: { value: 'B' },
          answer_version: 1,
          updated_at: '2026-07-19T09:30:00Z',
        },
      ],
      initialAttemptVersion: 4,
    })
    expect(engine.getStatus(Q1)).toBe('saved')
    await vi.advanceTimersByTimeAsync(5000)
    expect(calls).toHaveLength(0)
    engine.destroy()
  })
})

describe('flush', () => {
  it('flushQuestion 跳过 debounce 立即保存', async () => {
    const { engine, calls } = createEngine((qid) => Promise.resolve(savedAnswer(qid, 1, 2)))
    engine.setLocal(Q1, 'A')
    engine.flushQuestion(Q1)
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    engine.destroy()
  })

  it('flushAll 等待进行中的保存链完成；有 conflict 时返回 false', async () => {
    let resolveQ2: ((value: SavedAnswer) => void) | undefined
    const { engine } = createEngine((qid) => {
      if (qid === Q1) {
        return Promise.reject(conflictProblem({ value: 'X', answerVersion: 2, attemptVersion: 6 }))
      }
      return new Promise<SavedAnswer>((resolve) => {
        resolveQ2 = resolve
      })
    })

    engine.setLocal(Q1, 'A')
    engine.setLocal(Q2, '短文')
    await vi.advanceTimersByTimeAsync(800)

    let flushed: boolean | undefined
    const flushPromise = engine.flushAll().then((ok) => {
      flushed = ok
    })
    await vi.advanceTimersByTimeAsync(0)
    expect(flushed).toBeUndefined() // Q2 仍在保存中，flushAll 未返回

    resolveQ2?.(savedAnswer(Q2, 1, 7))
    await flushPromise
    expect(flushed).toBe(false) // Q1 conflict
    expect(engine.getStatus(Q2)).toBe('saved')
    expect(engine.getStatus(Q1)).toBe('conflict')
    engine.destroy()
  })
})
