-- 0001_init: AI Study Tutor 初始 schema。
-- 依据 docs/web-rebuild/03-backend-architecture.md 第 6 节数据模型。
-- 所有时间字段 TIMESTAMPTZ；可更新资源带 version BIGINT 乐观锁。

-- =========================================================================
-- 6.1 课程与内容
-- =========================================================================

CREATE TABLE courses (
    id          UUID PRIMARY KEY,
    slug        VARCHAR(220) NOT NULL UNIQUE,
    title       VARCHAR(200) NOT NULL CHECK (length(btrim(title)) > 0),
    description TEXT,
    archived    BOOLEAN NOT NULL DEFAULT false,
    version     BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE lessons (
    id          UUID PRIMARY KEY,
    course_id   UUID NOT NULL REFERENCES courses (id),
    ordinal     INTEGER NOT NULL CHECK (ordinal > 0),
    title       VARCHAR(200) NOT NULL CHECK (length(btrim(title)) > 0),
    description TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}',
    status      TEXT NOT NULL DEFAULT 'draft'
                CHECK (status IN ('draft', 'processing', 'ready', 'failed', 'ready_with_warning')),
    version     BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (course_id, ordinal)
);

CREATE TABLE file_objects (
    id                   UUID PRIMARY KEY,
    storage_key          TEXT NOT NULL UNIQUE,
    original_name        TEXT,
    media_type           TEXT,
    size_bytes           BIGINT CHECK (size_bytes >= 0),
    sha256               CHAR(64) CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'),
    state                TEXT NOT NULL
                         CHECK (state IN ('temporary', 'ready', 'quarantined', 'deleted')),
    temporary_owner_type TEXT CHECK (temporary_owner_type IS NULL OR temporary_owner_type IN ('job', 'idempotency')),
    temporary_owner_id   UUID,
    temporary_expires_at TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- temporary 行必须携带 Owner 与过期时间；其他状态不得残留 Owner。
    CHECK (
        (state = 'temporary'
            AND temporary_owner_type IS NOT NULL
            AND temporary_owner_id IS NOT NULL
            AND temporary_expires_at IS NOT NULL)
        OR
        (state <> 'temporary'
            AND temporary_owner_type IS NULL
            AND temporary_owner_id IS NULL)
    )
);

-- 文件 Recovery 扫描：过期 temporary 行。
CREATE INDEX idx_file_objects_temporary_expiry
    ON file_objects (temporary_expires_at)
    WHERE state = 'temporary';

CREATE TABLE materials (
    id             UUID PRIMARY KEY,
    lesson_id      UUID NOT NULL REFERENCES lessons (id),
    kind           TEXT NOT NULL CHECK (kind IN ('url', 'upload', 'text')),
    source_url     TEXT,
    file_object_id UUID REFERENCES file_objects (id),
    inline_text    TEXT,
    metadata       JSONB NOT NULL DEFAULT '{}',
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'failed')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- 不同 kind 的字段组合不变量。
    CHECK (
        (kind = 'url' AND source_url IS NOT NULL AND file_object_id IS NULL AND inline_text IS NULL)
        OR (kind = 'upload' AND file_object_id IS NOT NULL AND source_url IS NULL AND inline_text IS NULL)
        OR (kind = 'text' AND inline_text IS NOT NULL AND source_url IS NULL AND file_object_id IS NULL)
    )
);

-- =========================================================================
-- 6.4 任务与 AI 审计（jobs 先于 documents/questions 建表，供 source_job_id 引用）
-- =========================================================================

CREATE TABLE jobs (
    id                 UUID PRIMARY KEY,
    kind               TEXT NOT NULL CHECK (kind IN (
                           'claude_connectivity_test',
                           'lesson_import',
                           'lesson_regenerate',
                           'quiz_generate',
                           'review_quiz_generate',
                           'attempt_grade',
                           'wrong_answer_analyze',
                           'summary_evaluate',
                           'lesson_export')),
    aggregate_type     TEXT NOT NULL,
    aggregate_id       UUID NOT NULL,
    generation_family  TEXT CHECK (generation_family IS NULL OR generation_family = 'lesson_generation'),
    status             TEXT NOT NULL DEFAULT 'queued'
                       CHECK (status IN ('queued', 'running', 'validating', 'cancel_requested',
                                         'succeeded', 'failed', 'cancelled')),
    stage              TEXT NOT NULL DEFAULT 'queued',
    progress           SMALLINT NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    payload            JSONB NOT NULL DEFAULT '{}',
    result             JSONB,
    error_code         TEXT,
    error_detail       TEXT,
    priority           INTEGER NOT NULL DEFAULT 0,
    attempt_count      INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts       INTEGER NOT NULL DEFAULT 3 CHECK (max_attempts > 0),
    available_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_owner        TEXT,
    lease_expires_at   TIMESTAMPTZ,
    cancel_requested   BOOLEAN NOT NULL DEFAULT false,
    retry_of_job_id    UUID REFERENCES jobs (id),
    idempotency_scope  TEXT,
    idempotency_key    TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at         TIMESTAMPTZ,
    completed_at       TIMESTAMPTZ,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- 租约字段必须成对出现。
    CHECK ((lease_owner IS NULL) = (lease_expires_at IS NULL)),
    -- 幂等字段必须成对出现。
    CHECK ((idempotency_scope IS NULL) = (idempotency_key IS NULL))
);

-- 队列领取：queued 且可领取，按优先级/创建时间排序。
CREATE INDEX idx_jobs_claimable
    ON jobs (priority DESC, created_at ASC)
    WHERE status = 'queued';

-- 租约恢复扫描。
CREATE INDEX idx_jobs_lease_expiry
    ON jobs (lease_expires_at)
    WHERE status IN ('running', 'validating', 'cancel_requested');

-- 领域侧幂等第二道保护。
CREATE UNIQUE INDEX uq_jobs_idempotency
    ON jobs (idempotency_scope, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- 同一 Lesson 的 lesson_generation 族最多一个活动 Job（03 §6.4）。
CREATE UNIQUE INDEX uq_jobs_active_lesson_generation_family
    ON jobs (aggregate_id, generation_family)
    WHERE aggregate_type = 'lesson'
      AND generation_family IS NOT NULL
      AND status IN ('queued', 'running', 'validating', 'cancel_requested');

-- 同一 Attempt 最多一个活动批阅 Job（04 §10：提交与 grading-jobs 重建互斥，
-- “同一 Attempt 提交两次只产生一个批阅 Job”由本索引在并发下兜底）。
CREATE UNIQUE INDEX uq_jobs_active_attempt_grade
    ON jobs (aggregate_id)
    WHERE kind = 'attempt_grade'
      AND status IN ('queued', 'running', 'validating', 'cancel_requested');

CREATE TABLE job_events (
    id         BIGSERIAL PRIMARY KEY,
    job_id     UUID NOT NULL REFERENCES jobs (id),
    event_type TEXT NOT NULL CHECK (event_type IN (
                   'job.queued', 'job.started', 'job.progress', 'job.validating',
                   'job.cancel_requested', 'job.succeeded', 'job.failed', 'job.cancelled')),
    stage      TEXT NOT NULL,
    progress   SMALLINT NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    message    TEXT NOT NULL DEFAULT '',
    data       JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_events_job_id_id ON job_events (job_id, id);
CREATE INDEX idx_job_events_created_at ON job_events (created_at);

CREATE TABLE ai_runs (
    id                      UUID PRIMARY KEY,
    job_id                  UUID NOT NULL REFERENCES jobs (id),
    provider                TEXT NOT NULL DEFAULT 'anthropic',
    purpose                 TEXT NOT NULL,
    model                   TEXT NOT NULL,
    prompt_template_version TEXT NOT NULL,
    schema_version          TEXT NOT NULL DEFAULT '',
    prompt_hash             CHAR(64) CHECK (prompt_hash IS NULL OR prompt_hash ~ '^[0-9a-f]{64}$'),
    anthropic_request_id    TEXT,
    input_tokens            BIGINT CHECK (input_tokens >= 0),
    output_tokens           BIGINT CHECK (output_tokens >= 0),
    cache_read_tokens       BIGINT CHECK (cache_read_tokens >= 0),
    cache_write_tokens      BIGINT CHECK (cache_write_tokens >= 0),
    latency_ms              BIGINT CHECK (latency_ms >= 0),
    stop_reason             TEXT,
    redacted                BOOLEAN NOT NULL DEFAULT true,
    status                  TEXT NOT NULL CHECK (status IN ('success', 'error')),
    error_type              TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_runs_job_id ON ai_runs (job_id);

-- =========================================================================
-- 6.1 文档（依赖 jobs）
-- =========================================================================

CREATE TABLE documents (
    id            UUID PRIMARY KEY,
    lesson_id     UUID NOT NULL REFERENCES lessons (id),
    kind          TEXT NOT NULL CHECK (kind IN ('raw_text', 'student_notes', 'teacher_outline')),
    revision      INTEGER NOT NULL CHECK (revision > 0),
    content_md    TEXT NOT NULL,
    source_job_id UUID REFERENCES jobs (id),
    is_active     BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (lesson_id, kind, revision)
);

-- 每个 lesson+kind 只有一个 active 版本。
CREATE UNIQUE INDEX uq_documents_active
    ON documents (lesson_id, kind)
    WHERE is_active;

-- =========================================================================
-- 6.2 试卷、题库和作答
-- =========================================================================

CREATE TABLE questions (
    id              UUID PRIMARY KEY,
    lesson_id       UUID NOT NULL REFERENCES lessons (id),
    source_job_id   UUID REFERENCES jobs (id),
    type            TEXT NOT NULL CHECK (type IN (
                        'single_choice', 'multiple_choice', 'true_false',
                        'short_answer', 'case_analysis')),
    stem            TEXT NOT NULL,
    options         JSONB,
    correct_answer  JSONB NOT NULL,
    explanation     TEXT,
    tested_concepts JSONB NOT NULL DEFAULT '[]',
    difficulty      TEXT NOT NULL CHECK (difficulty IN ('easy', 'medium', 'hard', 'challenge')),
    cognitive_level TEXT NOT NULL CHECK (cognitive_level IN (
                        'recall', 'explain', 'apply', 'transfer', 'reflect')),
    default_score   NUMERIC(8, 2) NOT NULL CHECK (default_score > 0),
    content_hash    CHAR(64) CHECK (content_hash IS NULL OR content_hash ~ '^[0-9a-f]{64}$'),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_questions_lesson_id ON questions (lesson_id);

CREATE TABLE quizzes (
    id              UUID PRIMARY KEY,
    lesson_id       UUID NOT NULL REFERENCES lessons (id),
    title           VARCHAR(300) NOT NULL CHECK (length(btrim(title)) > 0),
    source          TEXT NOT NULL CHECK (source IN ('regular', 'retest', 'assembled')),
    policy_snapshot JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'ready', 'archived')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_quizzes_lesson_id ON quizzes (lesson_id);

CREATE TABLE quiz_items (
    quiz_id     UUID NOT NULL REFERENCES quizzes (id),
    question_id UUID NOT NULL REFERENCES questions (id),
    position    INTEGER NOT NULL CHECK (position > 0),
    score       NUMERIC(8, 2) NOT NULL CHECK (score > 0),
    PRIMARY KEY (quiz_id, question_id),
    UNIQUE (quiz_id, position)
);

CREATE TABLE attempts (
    id                   UUID PRIMARY KEY,
    quiz_id              UUID NOT NULL REFERENCES quizzes (id),
    status               TEXT NOT NULL DEFAULT 'in_progress'
                         CHECK (status IN ('in_progress', 'submitted', 'grading',
                                           'graded', 'grading_failed')),
    version              BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    auto_score           NUMERIC(8, 2) CHECK (auto_score IS NULL OR auto_score >= 0),
    final_score          NUMERIC(8, 2) CHECK (final_score IS NULL OR final_score >= 0),
    max_score            NUMERIC(8, 2) CHECK (max_score IS NULL OR max_score >= 0),
    grading_job_id       UUID REFERENCES jobs (id),
    grading_error_code   TEXT,
    grading_error_detail TEXT,
    started_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at         TIMESTAMPTZ,
    graded_at            TIMESTAMPTZ,
    -- 未提交不得有分数；未最终批阅不得有 final_score/graded_at（提交检查点约束见 B4 应用层）。
    CHECK ((submitted_at IS NULL) = (auto_score IS NULL)),
    CHECK ((graded_at IS NULL) = (final_score IS NULL))
);

CREATE INDEX idx_attempts_quiz_id ON attempts (quiz_id);

CREATE TABLE attempt_answers (
    attempt_id     UUID NOT NULL REFERENCES attempts (id),
    question_id    UUID NOT NULL REFERENCES questions (id),
    answer         JSONB,
    answer_version BIGINT NOT NULL DEFAULT 0 CHECK (answer_version >= 0),
    auto_correct   BOOLEAN,
    awarded_score  NUMERIC(8, 2) CHECK (awarded_score IS NULL OR awarded_score >= 0),
    feedback       JSONB,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (attempt_id, question_id)
);

-- =========================================================================
-- 6.3 学习记忆
-- =========================================================================

CREATE TABLE weaknesses (
    id            UUID PRIMARY KEY,
    course_id     UUID NOT NULL REFERENCES courses (id),
    lesson_id     UUID REFERENCES lessons (id),
    concept_key   VARCHAR(300) NOT NULL CHECK (length(btrim(concept_key)) > 0),
    concept_label VARCHAR(300) NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    evidence      JSONB NOT NULL DEFAULT '[]',
    severity      SMALLINT NOT NULL CHECK (severity BETWEEN 1 AND 5),
    review_count  INTEGER NOT NULL DEFAULT 0 CHECK (review_count >= 0),
    mastered      BOOLEAN NOT NULL DEFAULT false,
    version       BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 跨课时薄弱点用 zero UUID 归一化后参与唯一约束。
CREATE UNIQUE INDEX uq_weaknesses_concept
    ON weaknesses (course_id, COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid), concept_key);

CREATE TABLE mastery_snapshots (
    id                UUID PRIMARY KEY,
    scope_type        TEXT NOT NULL CHECK (scope_type IN ('course', 'lesson')),
    course_id         UUID NOT NULL REFERENCES courses (id),
    lesson_id         UUID REFERENCES lessons (id),
    formula_version   TEXT NOT NULL,
    value             NUMERIC(5, 2) NOT NULL CHECK (value BETWEEN 0 AND 100),
    components        JSONB NOT NULL DEFAULT '{}',
    source_attempt_id UUID REFERENCES attempts (id),
    recorded_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((scope_type = 'lesson') = (lesson_id IS NOT NULL))
);

CREATE INDEX idx_mastery_snapshots_scope
    ON mastery_snapshots (scope_type, course_id, lesson_id, recorded_at DESC);

-- 同一 Attempt/公式版本的重复消费不得生成第二条快照。
CREATE UNIQUE INDEX uq_mastery_snapshots_source
    ON mastery_snapshots (source_attempt_id, formula_version, scope_type, course_id,
                          COALESCE(lesson_id, '00000000-0000-0000-0000-000000000000'::uuid))
    WHERE source_attempt_id IS NOT NULL;

CREATE TABLE summary_evaluations (
    id           UUID PRIMARY KEY,
    lesson_id    UUID NOT NULL REFERENCES lessons (id),
    summary_text TEXT NOT NULL,
    result       JSONB,
    status       TEXT NOT NULL DEFAULT 'evaluating'
                 CHECK (status IN ('evaluating', 'completed', 'failed')),
    job_id       UUID REFERENCES jobs (id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

-- =========================================================================
-- 6.5 HTTP 幂等记录
-- =========================================================================

CREATE TABLE idempotency_records (
    id              UUID PRIMARY KEY,
    scope           TEXT NOT NULL,
    key             TEXT NOT NULL,
    method          TEXT NOT NULL,
    route_template  TEXT NOT NULL,
    request_hash    CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    state           TEXT NOT NULL DEFAULT 'in_progress' CHECK (state IN ('in_progress', 'completed')),
    response_status INTEGER CHECK (response_status IS NULL OR response_status BETWEEN 100 AND 599),
    response_body   JSONB,
    -- 需随回放返回的响应头（如 Location），JSON 对象；仅保存契约声明的头。
    response_headers JSONB,
    resource_type   TEXT,
    resource_id     UUID,
    job_id          UUID REFERENCES jobs (id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    UNIQUE (scope, key)
);

CREATE INDEX idx_idempotency_records_expiry ON idempotency_records (expires_at);

-- =========================================================================
-- 6.6 备份元数据 与 6.7 系统维护状态
-- =========================================================================

CREATE TABLE backups (
    id                UUID PRIMARY KEY,
    status            TEXT NOT NULL DEFAULT 'creating' CHECK (status IN ('creating', 'ready', 'failed')),
    maintenance_owner TEXT,
    app_version       TEXT NOT NULL DEFAULT '',
    db_version        TEXT NOT NULL DEFAULT '',
    display_name      TEXT NOT NULL DEFAULT '',
    size_bytes        BIGINT CHECK (size_bytes IS NULL OR size_bytes >= 0),
    manifest          JSONB,
    checksum          CHAR(64) CHECK (checksum IS NULL OR checksum ~ '^[0-9a-f]{64}$'),
    error_code        TEXT,
    error_detail      TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at      TIMESTAMPTZ
);

CREATE INDEX idx_backups_status_created ON backups (status, created_at DESC);

CREATE TABLE system_state (
    singleton                    BOOLEAN PRIMARY KEY CHECK (singleton),
    maintenance_mode             BOOLEAN NOT NULL DEFAULT false,
    phase                        TEXT NOT NULL DEFAULT 'normal'
                                   CHECK (phase IN ('normal', 'draining', 'quiesced')),
    maintenance_reason           TEXT,
    maintenance_owner            TEXT,
    maintenance_started_at       TIMESTAMPTZ,
    maintenance_lease_expires_at TIMESTAMPTZ,
    maintenance_heartbeat_at     TIMESTAMPTZ,
    version                      BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO system_state (singleton) VALUES (true);

-- =========================================================================
-- 6.8 旧系统导入审计
-- =========================================================================

CREATE TABLE legacy_import_batches (
    id                   UUID PRIMARY KEY,
    source_manifest_hash CHAR(64) NOT NULL CHECK (source_manifest_hash ~ '^[0-9a-f]{64}$'),
    importer_version     TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'scanned'
                         CHECK (status IN ('scanned', 'importing', 'needs_review', 'completed', 'failed')),
    report               JSONB,
    started_at           TIMESTAMPTZ,
    completed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE legacy_import_records (
    id            UUID PRIMARY KEY,
    batch_id      UUID NOT NULL REFERENCES legacy_import_batches (id),
    source_path   TEXT NOT NULL,
    source_sha256 CHAR(64) NOT NULL CHECK (source_sha256 ~ '^[0-9a-f]{64}$'),
    source_type   TEXT NOT NULL,
    target_type   TEXT NOT NULL,
    target_id     UUID,
    status        TEXT NOT NULL
                  CHECK (status IN ('imported', 'deduplicated', 'quarantined', 'skipped_by_policy')),
    reason_code   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (batch_id, source_path, target_type)
);
