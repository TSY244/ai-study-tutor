/**
 * 任务事件流活性诊断：内存快照 + 订阅（与 diagnostics.ts 同构）。
 * 只记录连接元数据（状态、游标、心跳、事件计数、最近错误），
 * 不记录事件正文，不做遥测外发。供设置页开发模式诊断面板与
 * 断线提示读取。
 */

export type JobStreamState = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed'

export interface JobStreamStatus {
  state: JobStreamState
  /** 最近已确认的持久化事件游标（Last-Event-ID 来源）。 */
  lastEventId: string | null
  lastHeartbeatAt: string | null
  lastEventAt: string | null
  /** 最近一次 stream.resync_required 触发 REST 对账的时间。 */
  lastResyncAt: string | null
  reconnectAttempts: number
  eventCounts: Record<string, number>
  lastError: string | null
}

const initialStatus: JobStreamStatus = {
  state: 'idle',
  lastEventId: null,
  lastHeartbeatAt: null,
  lastEventAt: null,
  lastResyncAt: null,
  reconnectAttempts: 0,
  eventCounts: {},
  lastError: null,
}

let snapshot: JobStreamStatus = { ...initialStatus }
const listeners = new Set<() => void>()

function emit(): void {
  for (const listener of listeners) listener()
}

function update(patch: Partial<JobStreamStatus>): void {
  snapshot = { ...snapshot, ...patch }
  emit()
}

export function getJobStreamStatus(): JobStreamStatus {
  return snapshot
}

export function subscribeJobStreamStatus(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function setJobStreamState(state: JobStreamState, reconnectAttempts?: number): void {
  update({
    state,
    reconnectAttempts: reconnectAttempts ?? snapshot.reconnectAttempts,
    lastError: state === 'open' ? null : snapshot.lastError,
  })
}

/** 持久化 Job 事件：推进面板游标与计数（业务游标由 JobStreamClient 维护）。 */
export function recordJobStreamEvent(event: string, id: string | null): void {
  update({
    lastEventId: id ?? snapshot.lastEventId,
    lastEventAt: new Date().toISOString(),
    eventCounts: { ...snapshot.eventCounts, [event]: (snapshot.eventCounts[event] ?? 0) + 1 },
  })
}

/** 心跳只证明连接活性：不写业务缓存，也不改变游标。 */
export function recordJobStreamHeartbeat(serverTime: string | null): void {
  update({
    lastHeartbeatAt: serverTime ?? new Date().toISOString(),
    eventCounts: { ...snapshot.eventCounts, heartbeat: (snapshot.eventCounts.heartbeat ?? 0) + 1 },
  })
}

export function recordJobStreamResync(highWatermarkEventId: string): void {
  update({
    lastEventId: highWatermarkEventId,
    lastResyncAt: new Date().toISOString(),
    eventCounts: {
      ...snapshot.eventCounts,
      'stream.resync_required': (snapshot.eventCounts['stream.resync_required'] ?? 0) + 1,
    },
  })
}

export function recordJobStreamError(message: string): void {
  update({ lastError: message })
}

/** 测试用：清空快照。 */
export function resetJobStreamStatus(): void {
  snapshot = { ...initialStatus, eventCounts: {} }
  emit()
}
