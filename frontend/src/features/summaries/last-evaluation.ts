/**
 * 每个课时最近一次总结评估 id 的 localStorage 记录。
 *
 * 保存用户最近查看的 evaluationId，返回时按 id 重取结果。
 * 存储不可用（隐私模式）时静默降级：刷新后不再回显上次结果。
 */

export const LAST_EVALUATION_STORAGE_PREFIX = 'summary-evaluation:'

function storage(): Storage | null {
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

/** 读取某课时最近一次的评估 id；不存在返回 null。 */
export function loadLastEvaluationId(lessonId: string): string | null {
  try {
    return storage()?.getItem(LAST_EVALUATION_STORAGE_PREFIX + lessonId) ?? null
  } catch {
    return null
  }
}

/** 提交评估成功后记录评估 id（写失败不影响评估本身）。 */
export function saveLastEvaluationId(lessonId: string, evaluationId: string): void {
  try {
    storage()?.setItem(LAST_EVALUATION_STORAGE_PREFIX + lessonId, evaluationId)
  } catch {
    // 配额/隐私模式：降级为不回显
  }
}

/** 删除评估后忘掉该课时的选择。 */
export function forgetLastEvaluationId(lessonId: string): void {
  try {
    storage()?.removeItem(LAST_EVALUATION_STORAGE_PREFIX + lessonId)
  } catch {
    /* 隐私模式 */
  }
}

/** 测试隔离：清空全部课时的评估 id 记录。 */
export function resetLastEvaluationIds(): void {
  const store = storage()
  if (!store) return
  const doomed: string[] = []
  for (let index = 0; index < store.length; index += 1) {
    const key = store.key(index)
    if (key?.startsWith(LAST_EVALUATION_STORAGE_PREFIX)) doomed.push(key)
  }
  for (const key of doomed) store.removeItem(key)
}
