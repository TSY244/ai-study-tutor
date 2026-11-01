/**
 * 最近学习记录的 localStorage 持久化（学习总览「继续学习」）。
 *
 * 最近访问属于纯客户端状态，服务端不感知；打开课时工作区时写入，
 * 按 visitedAt 倒序保留最近 5 条，同一课时重复访问只保留最新一条。
 *
 * 格式（key: `recent-learning`）：
 * ```json
 * {
 *   "version": 1,
 *   "entries": [
 *     { "courseId": "...", "lessonId": "...", "lessonTitle": "...", "visitedAt": "..." }
 *   ]
 * }
 * ```
 *
 * localStorage 不可用（隐私模式/配额）时静默降级：不阻断学习流程，
 * 「继续学习」退化为空状态。
 */

export interface RecentLearningEntry {
  courseId: string
  lessonId: string
  lessonTitle: string
  visitedAt: string
}

interface RecentLearningPayload {
  version: 1
  entries: RecentLearningEntry[]
}

export const RECENT_LEARNING_STORAGE_KEY = 'recent-learning'

/** 「继续学习」最多展示的条数。 */
export const RECENT_LEARNING_LIMIT = 5

function storage(): Storage | null {
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

function isEntryShape(value: unknown): value is RecentLearningEntry {
  if (!value || typeof value !== 'object') return false
  const entry = value as Partial<RecentLearningEntry>
  return (
    typeof entry.courseId === 'string' &&
    typeof entry.lessonId === 'string' &&
    typeof entry.lessonTitle === 'string' &&
    typeof entry.visitedAt === 'string'
  )
}

/** 读取最近学习记录；不存在或结构损坏时返回空数组（损坏内容会被清除）。 */
export function loadRecentLearning(): RecentLearningEntry[] {
  const store = storage()
  if (!store) return []
  const raw = store.getItem(RECENT_LEARNING_STORAGE_KEY)
  if (!raw) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    if (
      parsed &&
      typeof parsed === 'object' &&
      (parsed as Partial<RecentLearningPayload>).version === 1 &&
      Array.isArray((parsed as Partial<RecentLearningPayload>).entries)
    ) {
      return (parsed as RecentLearningPayload).entries
        .filter(isEntryShape)
        .slice(0, RECENT_LEARNING_LIMIT)
    }
  } catch {
    // JSON 损坏：按无效处理
  }
  store.removeItem(RECENT_LEARNING_STORAGE_KEY)
  return []
}

/** 记录一次课时访问（打开课时工作区时调用；写失败不阻塞学习）。 */
export function recordRecentLearning(entry: {
  courseId: string
  lessonId: string
  lessonTitle: string
}): void {
  try {
    const entries: RecentLearningEntry[] = [
      { ...entry, visitedAt: new Date().toISOString() },
      ...loadRecentLearning().filter((item) => item.lessonId !== entry.lessonId),
    ].slice(0, RECENT_LEARNING_LIMIT)
    const payload: RecentLearningPayload = { version: 1, entries }
    storage()?.setItem(RECENT_LEARNING_STORAGE_KEY, JSON.stringify(payload))
  } catch {
    // 配额/隐私模式：降级为不记录
  }
}

/** 测试隔离：清空最近学习记录。 */
export function resetRecentLearning(): void {
  try {
    storage()?.removeItem(RECENT_LEARNING_STORAGE_KEY)
  } catch {
    // 与写入同理，清除失败不阻塞
  }
}

/**
 * 清理指向已删除课程/课时的记录：删除课程/课时成功后调用，
 * 避免「继续学习」残留指向 404 的条目。与写入同理，失败不阻塞。
 */
export function removeRecentLearningEntries(filter: {
  courseId?: string
  lessonId?: string
}): void {
  try {
    const entries = loadRecentLearning().filter(
      (entry) =>
        (!filter.courseId || entry.courseId !== filter.courseId) &&
        (!filter.lessonId || entry.lessonId !== filter.lessonId),
    )
    const store = storage()
    if (!store) return
    if (entries.length === 0) {
      store.removeItem(RECENT_LEARNING_STORAGE_KEY)
      return
    }
    const payload: RecentLearningPayload = { version: 1, entries }
    store.setItem(RECENT_LEARNING_STORAGE_KEY, JSON.stringify(payload))
  } catch {
    // 与写入同理，清除失败不阻塞
  }
}
