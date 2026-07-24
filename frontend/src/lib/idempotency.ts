/**
 * Idempotency-Key：前端为一次用户意图生成一个 UUID v4（契约 04-api-contract.md §2.3）。
 *
 * 两层 API：
 * 1. 原语：createIdempotencyKey / isIdempotencyKey。
 * 2. 意图生命周期（begin/peek/resolve/rotate）：以 scope 标识一次创建/任务意图，
 *    同一意图的网络重试、双击防重、刷新恢复复用同一 Key；意图结束或载荷必须
 *    变化时才清除/轮换。Key 持久化在 sessionStorage：刷新可恢复，且天然限定在
 *    单标签页会话内（单用户本机应用，无跨标签页语义）。
 */

export function createIdempotencyKey(): string {
  return globalThis.crypto.randomUUID()
}

const UUID_V4_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

export function isIdempotencyKey(value: string): boolean {
  return UUID_V4_PATTERN.test(value)
}

/** sessionStorage Key 前缀；测试恢复路径时直接引用。 */
export const IDEMPOTENCY_STORAGE_PREFIX = 'idempotency-intent:'

// sessionStorage 不可用（隐私模式、非浏览器环境）时的内存镜像；
// 同一次页面会话内仍保证同一意图复用同一 Key。
const memoryIntents = new Map<string, string>()

function storage(): Storage | null {
  try {
    return globalThis.sessionStorage ?? null
  } catch {
    return null
  }
}

function readIntent(scope: string): string | undefined {
  const store = storage()
  const stored = store?.getItem(IDEMPOTENCY_STORAGE_PREFIX + scope)
  if (stored && isIdempotencyKey(stored)) return stored
  if (stored) store?.removeItem(IDEMPOTENCY_STORAGE_PREFIX + scope)
  return memoryIntents.get(scope)
}

function writeIntent(scope: string, key: string): void {
  memoryIntents.set(scope, key)
  try {
    storage()?.setItem(IDEMPOTENCY_STORAGE_PREFIX + scope, key)
  } catch {
    // 写入失败（配额/隐私模式）时退化为内存复用，不阻塞用户操作。
  }
}

/** 开始（或继续）一次意图：已存在则复用同一 Key，否则生成并持久化。 */
export function beginIdempotencyIntent(scope: string): string {
  return readIntent(scope) ?? rotateIdempotencyIntent(scope)
}

/** 查看意图当前 Key（不创建）；无意图时返回 undefined。 */
export function peekIdempotencyIntent(scope: string): string | undefined {
  return readIntent(scope)
}

/** 意图成功结束：清除 Key，下一次同 scope 的提交是全新意图。 */
export function resolveIdempotencyIntent(scope: string): void {
  memoryIntents.delete(scope)
  try {
    storage()?.removeItem(IDEMPOTENCY_STORAGE_PREFIX + scope)
  } catch {
    // 与写入同理，清除失败不阻塞。
  }
}

/**
 * 载荷必须变化时轮换 Key（如 validation_error 后用户修正表单）：
 * 同一 Key 携带不同内容会被服务端判为 idempotency_conflict。
 */
export function rotateIdempotencyIntent(scope: string): string {
  const key = createIdempotencyKey()
  writeIntent(scope, key)
  return key
}

/** 测试隔离用：清空全部意图（内存 + sessionStorage 前缀）。 */
export function resetIdempotencyIntents(): void {
  memoryIntents.clear()
  const store = storage()
  if (!store) return
  const doomed: string[] = []
  for (let index = 0; index < store.length; index += 1) {
    const key = store.key(index)
    if (key?.startsWith(IDEMPOTENCY_STORAGE_PREFIX)) doomed.push(key)
  }
  for (const key of doomed) store.removeItem(key)
}
