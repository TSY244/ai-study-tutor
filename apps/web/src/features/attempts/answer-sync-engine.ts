import type { AnswerSave, AttemptAnswer, SavedAnswer } from '@/api/generated/model'

import { isApiProblemError } from '@/api/problem'

import { answerValuesEqual } from './answer-value'
import {
  clearAttemptDraft,
  loadAttemptDraft,
  saveAttemptDraft,
  type DraftAnswerEntry,
} from './attempt-draft'

/**
 * 单题自动保存状态机（契约 04-api-contract.md §10.3，handoff §6.4）。
 *
 * 不变量：
 * - 输入立即写入内存与 sessionStorage；停止输入 debounceMs（默认 800ms）后 PATCH。
 * - 同一题保存串行（每题一条保存链），不同题并发。
 * - 请求携带该题当前 answer_version（未保存过为 0）；成功回写
 *   answer_version / attempt_version；全局 attempt_version 乱序取最大值。
 * - 409 version_conflict：current_answer 与本次发送值相等 ⇒ 先前的保存实际
 *   已成功（响应丢失/乱序），采用服务端版本继续；否则进入 conflict 等用户
 *   选择，绝不静默覆盖。
 * - 其他失败（network_error/5xx）⇒ unsynced，dirty 保留，由 flush/重试恢复；
 *   UI 必须显式显示「未同步」。
 */
export type QuestionSyncStatus = 'saved' | 'dirty' | 'saving' | 'unsynced' | 'conflict'

/** 全局同步状态（状态条展示用），优先级：conflict > unsynced > saving > saved。 */
export type OverallSyncStatus = 'saved' | 'saving' | 'unsynced' | 'conflict'

export interface ConflictSnapshot {
  questionId: string
  localValue: unknown
  serverValue: unknown
  serverAnswerVersion: number
  serverAttemptVersion: number
}

interface QuestionState {
  /** 当前显示值（本地最新输入）。 */
  value: unknown
  /** 服务端确认过的 answer_version；0 = 从未保存。 */
  savedVersion: number
  /** 本地值领先服务端（有待保存修改）。 */
  dirty: boolean
  status: QuestionSyncStatus
  clientUpdatedAt: string
  conflict: ConflictSnapshot | null
}

export interface AnswerSyncEngineOptions {
  attemptId: string
  /** 试题 ID 列表（快照顺序）。 */
  questionIds: string[]
  /** GET Attempt 返回的已保存答案（权威基线）。 */
  initialAnswers: AttemptAnswer[]
  /** GET Attempt 返回的全局 version。 */
  initialAttemptVersion: number
  saveFn: (questionId: string, body: AnswerSave) => Promise<SavedAnswer>
  /** 状态变化回调（React 用于触发重渲染）。 */
  onChange?: () => void
  debounceMs?: number
  now?: () => Date
}

const DEFAULT_DEBOUNCE_MS = 800

export class AnswerSyncEngine {
  private readonly attemptId: string
  private readonly saveFn: AnswerSyncEngineOptions['saveFn']
  private readonly debounceMs: number
  private readonly now: () => Date
  private onChange: (() => void) | undefined

  private readonly states = new Map<string, QuestionState>()
  private readonly timers = new Map<string, ReturnType<typeof setTimeout>>()
  private readonly chains = new Map<string, Promise<void>>()
  private attemptVersion: number

  constructor(options: AnswerSyncEngineOptions) {
    this.attemptId = options.attemptId
    this.saveFn = options.saveFn
    this.debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS
    this.now = options.now ?? (() => new Date())
    this.onChange = options.onChange

    const draft = loadAttemptDraft(options.attemptId)
    this.attemptVersion = Math.max(options.initialAttemptVersion, draft?.attemptVersion ?? 0)

    for (const questionId of options.questionIds) {
      const server = options.initialAnswers.find((answer) => answer.question_id === questionId)
      const state: QuestionState = {
        value: server?.answer.value,
        savedVersion: server?.answer_version ?? 0,
        dirty: false,
        status: 'saved',
        clientUpdatedAt: server?.updated_at ?? this.now().toISOString(),
        conflict: null,
      }
      const draftEntry = draft?.answers[questionId]
      const serverVersion = server?.answer_version ?? 0
      if (draftEntry && (draftEntry.dirty || draftEntry.savedVersion > serverVersion)) {
        // 恢复本地领先的部分：dirty（未同步修改）或草稿版本比服务端新
        // （上次保存的响应丢失/服务端状态较旧）。版本基线始终以服务端为准，
        // 值一致时直接视为已同步。
        if (server && answerValuesEqual(server.answer.value, draftEntry.value)) {
          state.value = server.answer.value
        } else {
          state.value = draftEntry.value
          state.dirty = true
          state.status = 'dirty'
          state.clientUpdatedAt = draftEntry.clientUpdatedAt
        }
      }
      this.states.set(questionId, state)
    }
    this.persist()

    // 刷新/恢复后立即继续同步未保存的修改
    for (const [questionId, state] of this.states) {
      if (state.dirty) this.schedule(questionId)
    }
  }

  // ------------------------------------------------------------------
  // 读取
  // ------------------------------------------------------------------

  getValue(questionId: string): unknown {
    return this.states.get(questionId)?.value
  }

  getStatus(questionId: string): QuestionSyncStatus {
    return this.states.get(questionId)?.status ?? 'saved'
  }

  getConflict(questionId: string): ConflictSnapshot | null {
    return this.states.get(questionId)?.conflict ?? null
  }

  /** 第一个未解决的冲突（UI 一次展示一个）。 */
  getFirstConflict(): ConflictSnapshot | null {
    for (const state of this.states.values()) {
      if (state.conflict) return state.conflict
    }
    return null
  }

  getAttemptVersion(): number {
    return this.attemptVersion
  }

  getOverallStatus(): OverallSyncStatus {
    let saving = false
    let unsynced = false
    for (const state of this.states.values()) {
      if (state.status === 'conflict') return 'conflict'
      if (state.status === 'unsynced') unsynced = true
      else if (state.dirty || state.status === 'saving' || state.status === 'dirty') saving = true
    }
    if (unsynced) return 'unsynced'
    return saving ? 'saving' : 'saved'
  }

  /** 提交前检查：全部答案与服务端一致（无 dirty/unsynced/conflict）。 */
  isFullySynced(): boolean {
    for (const state of this.states.values()) {
      if (state.dirty || state.status === 'conflict' || state.status === 'unsynced') return false
    }
    return true
  }

  // ------------------------------------------------------------------
  // 写入与同步
  // ------------------------------------------------------------------

  /** 用户输入：立即更新内存 + sessionStorage，并重置该题 debounce。 */
  setLocal(questionId: string, value: unknown): void {
    const state = this.states.get(questionId)
    if (!state || state.status === 'conflict') return
    state.value = value
    state.dirty = true
    state.clientUpdatedAt = this.now().toISOString()
    if (state.status !== 'saving') state.status = 'dirty'
    this.persist()
    this.notify()
    this.schedule(questionId)
  }

  /** 切题/输入失焦：跳过 debounce 立即保存该题。 */
  flushQuestion(questionId: string): void {
    this.clearTimer(questionId)
    const state = this.states.get(questionId)
    if (state?.dirty && state.status !== 'conflict') void this.requestSave(questionId)
  }

  /**
   * 提交前 flush：对所有待保存题目立即保存（同题串行、不同题并发），
   * 返回是否全部同步成功（false = 仍有 unsynced/conflict，禁止提交）。
   */
  async flushAll(): Promise<boolean> {
    for (const questionId of this.states.keys()) this.clearTimer(questionId)
    const runs: Promise<void>[] = []
    for (const [questionId, state] of this.states) {
      if (state.dirty && state.status !== 'conflict') {
        // unsynced 视为待重试的 dirty：保存链只处理 dirty/saving 状态
        if (state.status === 'unsynced') state.status = 'dirty'
        runs.push(this.requestSave(questionId))
      }
    }
    await Promise.all(runs)
    return this.isFullySynced()
  }

  /** 手动重试未同步题目（状态条「重试」按钮）。 */
  retryUnsynced(): void {
    void this.flushAll()
    this.notify()
  }

  /**
   * 冲突决策：'server' 采用服务端值与版本（丢弃本地修改）；
   * 'local' 以服务端版本为新基线重发本地值（用户明确选择的覆盖）。
   */
  resolveConflict(questionId: string, choice: 'local' | 'server'): void {
    const state = this.states.get(questionId)
    const conflict = state?.conflict
    if (!state || !conflict) return
    state.conflict = null
    state.savedVersion = conflict.serverAnswerVersion
    this.bumpAttemptVersion(conflict.serverAttemptVersion)
    if (choice === 'server') {
      state.value = conflict.serverValue
      state.dirty = false
      state.status = 'saved'
    } else {
      state.dirty = true
      state.status = 'dirty'
    }
    this.persist()
    this.notify()
    if (choice === 'local') void this.requestSave(questionId)
  }

  /** 提交成功后调用：清除 sessionStorage 草稿。 */
  discardDraft(): void {
    clearAttemptDraft(this.attemptId)
  }

  destroy(): void {
    for (const questionId of this.timers.keys()) this.clearTimer(questionId)
    this.onChange = undefined
  }

  // ------------------------------------------------------------------
  // 内部：保存链（同题串行）与 debounce
  // ------------------------------------------------------------------

  private schedule(questionId: string): void {
    this.clearTimer(questionId)
    this.timers.set(
      questionId,
      setTimeout(() => {
        this.timers.delete(questionId)
        this.flushQuestion(questionId)
      }, this.debounceMs),
    )
  }

  private clearTimer(questionId: string): void {
    const timer = this.timers.get(questionId)
    if (timer !== undefined) {
      clearTimeout(timer)
      this.timers.delete(questionId)
    }
  }

  /** 每题一条保存链：进行中时复用同一 Promise，保证同题串行。 */
  private requestSave(questionId: string): Promise<void> {
    const existing = this.chains.get(questionId)
    if (existing) return existing
    const run = async () => {
      // 保存期间的新输入会使 dirty 保持 true，链循环到干净或受阻为止；
      // conflict/unsynced 必须退出——否则失败会变成无延迟热重试。
      for (;;) {
        const state = this.states.get(questionId)
        if (!state || !state.dirty || state.status === 'conflict' || state.status === 'unsynced') {
          return
        }
        await this.saveOne(questionId)
      }
    }
    const chain = run().finally(() => {
      if (this.chains.get(questionId) === chain) this.chains.delete(questionId)
    })
    this.chains.set(questionId, chain)
    return chain
  }

  private async saveOne(questionId: string): Promise<void> {
    const state = this.states.get(questionId)
    if (!state || !state.dirty || state.status === 'conflict') return

    state.status = 'saving'
    this.notify()

    const sentValue = state.value
    const body: AnswerSave = {
      answer: { value: sentValue },
      answer_version: state.savedVersion,
      client_updated_at: state.clientUpdatedAt,
    }
    try {
      const saved = await this.saveFn(questionId, body)
      state.savedVersion = saved.answer_version
      this.bumpAttemptVersion(saved.attempt_version)
      if (answerValuesEqual(state.value, sentValue)) {
        state.dirty = false
        state.status = 'saved'
      } else {
        // 保存期间用户继续输入：保持 dirty，保存链会接着发新版本
        state.dirty = true
        state.status = 'dirty'
      }
    } catch (error) {
      this.handleSaveError(questionId, state, sentValue, error)
    }
    this.persist()
    this.notify()
  }

  private handleSaveError(
    questionId: string,
    state: QuestionState,
    sentValue: unknown,
    error: unknown,
  ): void {
    if (isApiProblemError(error) && error.code === 'version_conflict') {
      const currentAnswer = error.getExtension<{ value?: unknown } | null>('current_answer')
      const currentValue = currentAnswer?.value
      const currentAnswerVersion =
        error.getExtension<number>('current_answer_version') ?? state.savedVersion
      const currentAttemptVersion = error.getExtension<number>('current_attempt_version')
      if (typeof currentAttemptVersion === 'number') {
        this.bumpAttemptVersion(currentAttemptVersion)
      }
      if (answerValuesEqual(currentValue, sentValue)) {
        // 服务端已有本次发送的值：先前的保存实际已成功（响应丢失/乱序），
        // 采用服务端版本号继续；若用户又改过，保存链会用新版本重发。
        state.savedVersion = currentAnswerVersion
        if (answerValuesEqual(state.value, sentValue)) {
          state.dirty = false
          state.status = 'saved'
        } else {
          state.dirty = true
          state.status = 'dirty'
        }
        return
      }
      // 无法安全判断：交给用户选择，绝不静默覆盖
      state.status = 'conflict'
      state.conflict = {
        questionId,
        localValue: state.value,
        serverValue: currentValue,
        serverAnswerVersion: currentAnswerVersion,
        serverAttemptVersion: currentAttemptVersion ?? this.attemptVersion,
      }
      return
    }
    // network_error / 5xx / 其他：保留 dirty，显式未同步
    state.status = 'unsynced'
  }

  private bumpAttemptVersion(version: number): void {
    if (version > this.attemptVersion) this.attemptVersion = version
  }

  private persist(): void {
    const answers: Record<string, DraftAnswerEntry> = {}
    for (const [questionId, state] of this.states) {
      answers[questionId] = {
        value: state.value,
        savedVersion: state.savedVersion,
        dirty: state.dirty,
        clientUpdatedAt: state.clientUpdatedAt,
      }
    }
    saveAttemptDraft({
      version: 1,
      attemptId: this.attemptId,
      attemptVersion: this.attemptVersion,
      answers,
      updatedAt: this.now().toISOString(),
    })
  }

  private notify(): void {
    this.onChange?.()
  }
}
