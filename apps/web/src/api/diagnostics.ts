/**
 * 请求诊断：内存环形缓冲，记录最近 N 次请求的方法、路径、状态、耗时和
 * X-Request-ID，供开发模式诊断面板与错误页展示。只保留元数据，
 * 不记录请求/响应正文，不做遥测外发。
 */

export interface RequestDiagnostic {
  requestId: string
  method: string
  /** 同源相对路径，含查询串；不含主机。 */
  url: string
  status: number
  /** 问题码（无错误时为 undefined）。 */
  code?: string
  durationMs: number
  at: string
}

const MAX_ENTRIES = 50
const entries: RequestDiagnostic[] = []
const listeners = new Set<() => void>()

export function recordDiagnostic(entry: RequestDiagnostic): void {
  entries.unshift(entry)
  if (entries.length > MAX_ENTRIES) entries.length = MAX_ENTRIES
  for (const listener of listeners) listener()
}

export function getDiagnostics(): readonly RequestDiagnostic[] {
  return entries
}

export function subscribeDiagnostics(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function clearDiagnostics(): void {
  entries.length = 0
  for (const listener of listeners) listener()
}
