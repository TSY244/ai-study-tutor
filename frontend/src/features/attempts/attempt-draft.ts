/**
 * 作答草稿的 sessionStorage 持久化（刷新/崩溃恢复）。
 *
 * 格式（key: `attempt-draft:{attemptId}`）：
 * ```json
 * {
 *   "version": 1,
 *   "attemptId": "...",
 *   "attemptVersion": 8,
 *   "answers": {
 *     "<questionId>": {
 *       "value": "B",
 *       "savedVersion": 1,        // 本地已知的、服务端确认过的 answer_version（0 = 未保存）
 *       "dirty": true,            // 有未同步到服务端的修改
 *       "clientUpdatedAt": "..."
 *     }
 *   },
 *   "updatedAt": "..."
 * }
 * ```
 *
 * 恢复语义：以服务端 GET Attempt 为权威基线；草稿仅在上次会话有
 * 未同步修改（dirty）时覆盖显示值，版本基线始终取服务端 answer_version，
 * 因此恢复后的首次保存不会因版本过期而必然 409。
 * sessionStorage 不可用时静默降级为不持久化（内存草稿仍工作）。
 */

export interface DraftAnswerEntry {
  value: unknown
  savedVersion: number
  dirty: boolean
  clientUpdatedAt: string
}

export interface AttemptDraft {
  version: 1
  attemptId: string
  attemptVersion: number
  answers: Record<string, DraftAnswerEntry>
  updatedAt: string
}

export const ATTEMPT_DRAFT_STORAGE_PREFIX = 'attempt-draft:'

function storage(): Storage | null {
  try {
    return globalThis.sessionStorage ?? null
  } catch {
    return null
  }
}

function isDraftShape(value: unknown): value is AttemptDraft {
  if (!value || typeof value !== 'object') return false
  const draft = value as Partial<AttemptDraft>
  return (
    draft.version === 1 &&
    typeof draft.attemptId === 'string' &&
    typeof draft.answers === 'object' &&
    draft.answers !== null
  )
}

/** 读取草稿；不存在或结构损坏时返回 null（损坏项会被清除）。 */
export function loadAttemptDraft(attemptId: string): AttemptDraft | null {
  const store = storage()
  if (!store) return null
  const raw = store.getItem(ATTEMPT_DRAFT_STORAGE_PREFIX + attemptId)
  if (!raw) return null
  try {
    const parsed: unknown = JSON.parse(raw)
    if (isDraftShape(parsed) && parsed.attemptId === attemptId) return parsed
  } catch {
    // JSON 损坏：按无效处理
  }
  store.removeItem(ATTEMPT_DRAFT_STORAGE_PREFIX + attemptId)
  return null
}

/** 全量覆盖写草稿（引擎在每次状态变化后调用；写失败不阻塞作答）。 */
export function saveAttemptDraft(draft: AttemptDraft): void {
  try {
    storage()?.setItem(ATTEMPT_DRAFT_STORAGE_PREFIX + draft.attemptId, JSON.stringify(draft))
  } catch {
    // 配额/隐私模式：降级为仅内存草稿
  }
}

/** 作答提交成功（或用户明确放弃）后清除草稿。 */
export function clearAttemptDraft(attemptId: string): void {
  try {
    storage()?.removeItem(ATTEMPT_DRAFT_STORAGE_PREFIX + attemptId)
  } catch {
    // 与写入同理，清除失败不阻塞
  }
}

/** 测试隔离：清空全部作答草稿。 */
export function resetAttemptDrafts(): void {
  const store = storage()
  if (!store) return
  const doomed: string[] = []
  for (let index = 0; index < store.length; index += 1) {
    const key = store.key(index)
    if (key?.startsWith(ATTEMPT_DRAFT_STORAGE_PREFIX)) doomed.push(key)
  }
  for (const key of doomed) store.removeItem(key)
}
