-- Worker 心跳只保存非敏感运行能力；Claude Key 永不进入数据库。
CREATE TABLE worker_heartbeats (
    worker_id         TEXT PRIMARY KEY,
    claude_status     TEXT NOT NULL CHECK (claude_status IN ('ok', 'degraded', 'unavailable', 'unknown')),
    claude_message    TEXT,
    claude_model      TEXT,
    last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_worker_heartbeats_last_heartbeat
    ON worker_heartbeats (last_heartbeat_at DESC);
