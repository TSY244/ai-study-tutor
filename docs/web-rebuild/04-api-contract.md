# REST 与 SSE 接口文档

## 1. 契约原则

- 基础路径：`/api/v1`。
- 浏览器通过 Caddy 同源访问，不配置跨域 API 地址。
- OpenAPI 3.1 文件是接口唯一事实来源：[openapi.yaml](./openapi.yaml)。
- JSON 字段统一 `snake_case`。
- 普通成功响应直接返回资源；列表返回 `items + page_info`，不增加通用 `data` 外壳。
- 错误统一返回 `application/problem+json`。
- 长任务统一返回 `202 Accepted`，响应包含 Job。
- 创建资源/任务的 POST 支持 `Idempotency-Key`。
- 可更新资源携带 `version`，更新请求必须提交当前版本。
- API 不在任何响应中返回 Anthropic API Key。

## 2. 通用格式

### 2.1 ID 与时间

- ID：UUID 字符串。
- 时间：UTC RFC 3339，例如 `2026-07-17T08:30:00Z`。
- 分数：JSON number，后端数据库使用定点数。
- 空值：只有 Schema 允许时返回 `null`；不使用空字符串代替缺失对象。

### 2.2 Request ID

客户端可以发送：

```http
X-Request-ID: 01J...
```

服务端始终响应 `X-Request-ID`。若客户端值无效，服务端生成新值。错误响应体同时包含 `request_id`。

设计源文件顶层 `x-common-response-headers` 声明该 middleware 级不变量。实施时 `make generate-openapi` 必须先运行确定性的 Header 展开器，把 `X-Request-ID` 写入 `api/openapi.yaml` 的每一个成功和错误 Response，再执行代码生成；CI 校验展开产物不存在缺少该 Header 的响应。生成客户端外层必须保留原始响应 Header，不能只反序列化 JSON body。`docs/web-rebuild/openapi.yaml` 是设计源，展开后的 `api/openapi.yaml` 是服务端和前端代码生成的发布契约，二者由生成测试保证一致，禁止手工修改展开产物。

### 2.3 幂等键

用于创建课程、上传/导入素材、生成试卷、提交 Attempt、重试 Job 等操作：

```http
Idempotency-Key: 771f7e5f-1c92-4b8c-9ec4-a74ed6ef6bea
```

规则：

- 由前端为一次用户意图生成 UUID。
- 同一 API 资源范围内，相同 Key 和相同请求体返回第一次结果。
- 相同 Key 但请求体不同返回 `409 idempotency_conflict`。
- 首次请求仍在处理中时返回 `409 idempotency_in_progress` 和 `Retry-After`；客户端使用原 Key 重试，不能换 Key 创建重复资源。
- 服务端至少保存 7 天。

### 2.4 游标分页

请求：

```http
GET /api/v1/jobs?limit=20&cursor=eyJjcmVhdGVkX2F0...
```

响应：

```json
{
  "items": [],
  "page_info": {
    "next_cursor": null,
    "has_more": false
  }
}
```

`limit` 默认 20，最大 100。Cursor 为不透明字符串，前端不得解析。

### 2.5 乐观并发

资源更新请求携带 `version`：

```json
{
  "title": "极限与连续",
  "version": 3
}
```

版本不匹配返回 `409 version_conflict`，并在扩展字段提供当前版本；响应不自动覆盖客户端内容。

## 3. 错误格式

基于 Problem Details：

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/problem+json
```

```json
{
  "type": "https://ai-study-tutor.local/problems/validation-error",
  "title": "请求参数无效",
  "status": 422,
  "detail": "请修正标记的字段后重试。",
  "instance": "/api/v1/courses",
  "code": "validation_error",
  "request_id": "01J...",
  "errors": [
    {
      "field": "title",
      "code": "required",
      "message": "课程名称不能为空"
    }
  ]
}
```

稳定错误码：

| HTTP | `code` | 含义 |
| --- | --- | --- |
| 400 | `bad_request` | JSON、查询或 multipart 结构无效 |
| 404 | `not_found` | 资源不存在或已删除 |
| 409 | `version_conflict` | 乐观锁冲突 |
| 409 | `idempotency_conflict` | 幂等键与不同请求体重复使用 |
| 409 | `idempotency_in_progress` | 同一幂等请求仍在处理，按 `Retry-After` 重试 |
| 409 | `resource_busy` | 有运行任务，当前不能删除/修改 |
| 409 | `attempt_already_submitted` | Attempt 已提交 |
| 422 | `validation_error` | 字段校验失败 |
| 422 | `practice_not_needed` | 策略判断不需要继续生成普通练习 |
| 413 | `upload_too_large` | 上传超过限制 |
| 415 | `unsupported_media_type` | 文件类型不支持 |
| 429 | `too_many_requests` | 本地限流或重复操作过快 |
| 503 | `claude_unavailable` | Claude 暂时不可用 |
| 503 | `claude_not_configured` | 未配置有效的 Claude Key，非 AI 能力仍可使用 |
| 503 | `maintenance_mode` | 系统正在执行一致性备份等维护操作，暂时拒绝写入 |
| 507 | `storage_full` | 数据目录空间不足 |

Job 失败中的 `error_code` 使用更细粒度内部错误，参见后端文档。

## 4. System API

### 4.1 健康检查

| Method | Path | 用途 |
| --- | --- | --- |
| GET | `/healthz` | 进程存活，不检查外部依赖 |
| GET | `/readyz` | 数据库、迁移版本和必要存储可用 |

这些接口位于 API 根路径，不加 `/api/v1`。

### 4.2 系统状态

```http
GET /api/v1/system/status
```

```json
{
  "status": "degraded",
  "version": "0.3.0",
  "maintenance": {
    "enabled": false,
    "reason": null
  },
  "database": {
    "status": "ok",
    "migration_version": 12
  },
  "worker": {
    "status": "ok",
    "last_heartbeat_at": "2026-07-17T08:30:00Z",
    "active_jobs": 1,
    "queued_jobs": 2
  },
  "claude": {
    "status": "unknown",
    "model": "configured-model",
    "last_checked_at": null
  },
  "storage": {
    "status": "ok",
    "free_bytes": 21474836480
  }
}
```

`status` 为 `ok/degraded/unavailable`。Claude 状态由最近一次连接测试或实际调用更新。

### 4.3 Claude 连接测试

```http
POST /api/v1/system/claude-tests
Idempotency-Key: ...
```

API 无权读取 Claude Key，因此该操作创建 Worker Job：

```http
HTTP/1.1 202 Accepted
Location: /api/v1/jobs/{job_id}
```

```json
{
  "job": {
    "id": "...",
    "kind": "claude_connectivity_test",
    "status": "queued",
    "stage": "queued",
    "progress": 0
  }
}
```

## 5. Course API

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/courses` | 课程列表，支持 `archived` 和搜索 |
| POST | `/courses` | 创建课程 |
| GET | `/courses/{course_id}` | 课程详情和汇总 |
| PATCH | `/courses/{course_id}` | 修改标题、描述、归档状态 |
| DELETE | `/courses/{course_id}` | 删除课程；有活动任务时拒绝 |

创建课程：

```http
POST /api/v1/courses
Idempotency-Key: ...
Content-Type: application/json
```

```json
{
  "title": "高等数学",
  "description": "微积分基础"
}
```

```http
HTTP/1.1 201 Created
Location: /api/v1/courses/{course_id}
```

```json
{
  "id": "5d44786b-c956-43f8-a1ef-735877e80c85",
  "title": "高等数学",
  "description": "微积分基础",
  "archived": false,
  "lesson_count": 0,
  "version": 1,
  "created_at": "2026-07-17T08:30:00Z",
  "updated_at": "2026-07-17T08:30:00Z"
}
```

## 6. Lesson API

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/courses/{course_id}/lessons` | 课程课时列表 |
| POST | `/courses/{course_id}/lessons` | 创建空课时 |
| GET | `/lessons/{lesson_id}` | 课时详情、状态和汇总 |
| PATCH | `/lessons/{lesson_id}` | 修改名称、顺序和描述 |
| DELETE | `/lessons/{lesson_id}` | 删除课时；活动任务时拒绝 |

创建课时：

```json
{
  "ordinal": 1,
  "title": "极限的基本概念",
  "description": ""
}
```

Lesson 摘要：

```json
{
  "id": "...",
  "course_id": "...",
  "ordinal": 1,
  "title": "极限的基本概念",
  "status": "processing",
  "active_job_id": "...",
  "documents": {
    "raw_text": false,
    "student_notes": false,
    "teacher_outline": false
  },
  "mastery": null,
  "version": 2,
  "created_at": "...",
  "updated_at": "..."
}
```

## 7. Material API

### 7.1 URL 素材

```http
POST /api/v1/lessons/{lesson_id}/materials/url
Idempotency-Key: ...
Content-Type: application/json
```

```json
{
  "url": "https://www.youtube.com/watch?v=...",
  "preferred_language": "zh-CN"
}
```

仅接受 HTTPS 和后端允许列表中的站点；URL/DNS/重定向还要经过服务端 SSRF 校验。

### 7.2 文本素材

```http
POST /api/v1/lessons/{lesson_id}/materials/text
```

```json
{
  "title": "课堂讲义",
  "text": "完整学习材料..."
}
```

### 7.3 文件素材

```http
POST /api/v1/lessons/{lesson_id}/materials/upload
Content-Type: multipart/form-data
```

字段：

- `file`：必填，单文件。
- `preferred_language`：可选。
- `transcription_mode`：可选，`subtitle_only/whisper_local`。

上传响应只有在文件完整落盘并校验后才返回。三种素材接口均响应：

```http
HTTP/1.1 202 Accepted
Location: /api/v1/jobs/{job_id}
```

```json
{
  "material": {
    "id": "...",
    "lesson_id": "...",
    "kind": "url",
    "status": "pending",
    "created_at": "..."
  },
  "job": {
    "id": "...",
    "kind": "lesson_import",
    "status": "queued",
    "stage": "queued",
    "progress": 0
  }
}
```

补充接口：

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/lessons/{lesson_id}/materials` | 素材列表 |
| DELETE | `/materials/{material_id}` | 删除未被活动任务使用的素材 |

## 8. Document API

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/lessons/{lesson_id}/documents` | 当前文档摘要 |
| GET | `/lessons/{lesson_id}/documents/{kind}` | 当前发布文档 |
| GET | `/lessons/{lesson_id}/documents/{kind}/versions` | 历史版本 |
| GET | `/documents/{document_id}` | 指定版本 |
| POST | `/lessons/{lesson_id}/regenerations` | 重生成文档，返回 Job |

素材导入、文档重生成、普通出题和复习出题统一属于 `lesson_generation`。同一 Lesson 已有 `queued/running/validating/cancel_requested` 生成 Job 时，新请求返回 `409 resource_busy`，并在 Problem 扩展中返回 `active_job_id` 和 `generation_family=lesson_generation`。不同幂等键也不能创建第二个活动任务；前端应跳转或关联到现有 Job，而不是自动换 Key 重试。这样 `Lesson.active_job_id` 始终只有一个明确对象，Quiz 也不会与正在更新的内容并发生成。

`kind`：`raw_text/student_notes/teacher_outline`。

文档摘要和版本列表不返回 `content_md`；只有按 kind 查询当前文档或按 document ID 查询具体版本时返回正文，避免列表请求加载全部长文。

```json
{
  "id": "...",
  "lesson_id": "...",
  "kind": "student_notes",
  "revision": 2,
  "content_md": "# 极限的基本概念\n...",
  "source_job_id": "...",
  "is_active": true,
  "created_at": "..."
}
```

重生成请求：

```json
{
  "kinds": ["student_notes", "teacher_outline"],
  "reason": "根据最新素材重新生成"
}
```

旧 active 版本在新任务成功前继续可用。失败后 Lesson 进入 `ready_with_warning`，而不是丢失内容。

## 9. Quiz API

### 9.1 常规试卷生成

```http
POST /api/v1/lessons/{lesson_id}/quiz-generations
Idempotency-Key: ...
```

```json
{
  "question_count": null,
  "title": null
}
```

`question_count=null` 表示由策略决定。若策略返回不需继续练习，响应 `422 practice_not_needed`，不会创建空 Job。

### 9.2 复习试卷生成

```http
POST /api/v1/lessons/{lesson_id}/review-quiz-generations
```

```json
{
  "question_count": 8,
  "include_lesson_ids": ["optional-old-lesson-id"],
  "weakness_ids": []
}
```

这是独立资源和 Job 类型，保证 `source=retest`。

### 9.3 查询

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/lessons/{lesson_id}/quizzes` | 试卷列表 |
| GET | `/quizzes/{quiz_id}` | 试卷及题目 |
| POST | `/quizzes/{quiz_id}/attempts` | 开始新 Attempt |

学生试卷响应默认不返回 `correct_answer` 和 `explanation`。只有 Attempt 已提交后，相关反馈接口才返回允许展示的答案和解析。

## 10. Attempt API

### 10.1 开始作答

```http
POST /api/v1/quizzes/{quiz_id}/attempts
Idempotency-Key: ...
```

```http
HTTP/1.1 201 Created
Location: /api/v1/attempts/{attempt_id}
```

Attempt 创建时固定题目和分值快照。

### 10.2 查询 Attempt

```http
GET /api/v1/attempts/{attempt_id}
```

未提交时返回题目与已保存答案，不返回正确答案。已提交后根据批阅状态返回客观题结果或完整反馈。

### 10.3 自动保存单题答案

```http
PATCH /api/v1/attempts/{attempt_id}/answers/{question_id}
Content-Type: application/json
```

```json
{
  "answer": {
    "value": "B"
  },
  "answer_version": 0,
  "client_updated_at": "2026-07-17T08:35:10.123Z"
}
```

```json
{
  "attempt_id": "...",
  "question_id": "...",
  "answer": {
    "value": "B"
  },
  "answer_version": 1,
  "attempt_version": 8,
  "saved_at": "2026-07-17T08:35:10.200Z"
}
```

版本冲突：

```json
{
  "type": "https://ai-study-tutor.local/problems/version-conflict",
  "title": "答案版本冲突",
  "status": 409,
  "detail": "服务端已有更新的答案。",
  "code": "version_conflict",
  "request_id": "...",
  "current_answer_version": 2,
  "current_attempt_version": 9,
  "current_answer": {
    "value": "A"
  }
}
```

`answer_version` 是单题版本，首次保存传 0。同题写入按版本串行；不同题可以并发。每次成功写入还会递增 Attempt 全局版本，供最终提交使用。

### 10.4 提交

```http
POST /api/v1/attempts/{attempt_id}/submit
Idempotency-Key: ...
```

请求：

```json
{
  "version": 12
}
```

存在主观题时响应：

```http
HTTP/1.1 202 Accepted
```

```json
{
  "attempt": {
    "id": "...",
    "status": "grading",
    "auto_score": 6,
    "final_score": null,
    "max_score": 10,
    "version": 13,
    "submitted_at": "..."
  },
  "job": {
    "id": "...",
    "kind": "attempt_grade",
    "status": "queued",
    "stage": "queued",
    "progress": 0
  }
}
```

若试卷全是可本地判分的客观题，服务端同步完成并返回 `200`、`status=graded`，响应使用 `AttemptSubmitCompleted`，只包含 `attempt`，不包含 `job` 字段。存在主观题时返回 `202`，响应使用 `AttemptSubmitAccepted`，且 `job` 必须为非空 `attempt_grade` Job。前端必须按 HTTP 状态和生成类型分别处理，不能通过猜测可空字段判断。

### 10.5 批阅重试

批阅失败后使用 Job Retry，不创建新 Attempt：

```http
POST /api/v1/jobs/{job_id}/retry
```

这里的 `job_id` 是该 Attempt 当前失败的批阅任务 ID。

旧系统导入的失败/待处理批阅可能没有新系统 Job ID；不可重试 Job 或需要重新按当前 Prompt/Schema 批阅时，使用 Attempt 级入口：

```http
POST /api/v1/attempts/{attempt_id}/grading-jobs
Idempotency-Key: ...
```

该接口只接受 `grading_failed` Attempt，要求答案和不可变 Quiz 快照完整，且当前不存在活动批阅 Job。成功返回 `202 AsyncAccepted` 并把 Attempt 更新为 `grading`；若原 `grading_job_id` 存在，新 Job 的 `retry_of_job_id` 指向原 Job。重复幂等键返回同一 Job；已有活动 Job 返回 `409 resource_busy`；答案不完整或旧数据被隔离时返回稳定、可行动错误，不创建 Job。

## 11. Weakness 与 Progress API

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/weaknesses` | 支持 course、lesson、mastered、severity 过滤 |
| GET | `/weaknesses/{weakness_id}` | 薄弱点与证据 |
| POST | `/weaknesses/{weakness_id}/resolve` | 手动标记已掌握 |
| POST | `/weaknesses/{weakness_id}/reopen` | 重新激活 |
| GET | `/courses/{course_id}/progress` | 课程进度聚合 |
| GET | `/lessons/{lesson_id}/progress` | 课时掌握度和趋势 |

`resolve/reopen` 请求需要 `Idempotency-Key`，并携带当前 `version`；版本不一致返回 `409 version_conflict`。

聚合结果由后端的追加式 `mastery_snapshots` 计算，返回当前 `formula_version` 和带版本的趋势点；前端不重新实现掌握度公式。

## 12. Summary Evaluation API

```http
POST /api/v1/lessons/{lesson_id}/summary-evaluations
Idempotency-Key: ...
```

```json
{
  "summary": "极限描述函数在某一点附近的趋势……"
}
```

响应 `202`，返回 evaluation 占位资源和 Job。查询：

```http
GET /api/v1/summary-evaluations/{evaluation_id}
```

结果 Schema 至少包含：

```json
{
  "coverage_score": 0.78,
  "correct_points": [],
  "misconceptions": [],
  "missing_points": [],
  "suggestions": []
}
```

## 13. Job API

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/jobs` | 任务列表，按状态、活动状态、类型、资源过滤；`active=true` 与 `status` 互斥 |
| GET | `/jobs/{job_id}` | 任务详情 |
| GET | `/jobs/{job_id}/events` | 任务事件历史，游标分页 |
| GET | `/jobs/stream` | 全局任务 SSE |
| POST | `/jobs/{job_id}/cancel` | 请求取消 |
| POST | `/jobs/{job_id}/retry` | 为可重试的失败任务创建新 Job，并通过 `retry_of_job_id` 关联原任务 |

Job 示例：

```json
{
  "id": "...",
  "kind": "lesson_import",
  "aggregate_type": "lesson",
  "aggregate_id": "...",
  "status": "running",
  "stage": "generating_student_notes",
  "progress": 62,
  "message": "正在生成学生笔记",
  "attempt_count": 1,
  "max_attempts": 3,
  "retryable": false,
  "cancel_requested": false,
  "error": null,
  "created_at": "...",
  "started_at": "...",
  "completed_at": null,
  "updated_at": "..."
}
```

运行中的 `retryable=false` 表示当前不能重试，不代表失败后一定不可重试。

## 14. SSE 契约

```http
GET /api/v1/jobs/stream
Accept: text/event-stream
Last-Event-ID: 821
```

响应头：

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

事件：

```text
id: 822
event: job.progress
data: {"job_id":"...","status":"running","stage":"generating_student_notes","progress":62,"message":"正在生成学生笔记","occurred_at":"..."}

```

持久化 SSE 的 `data` 使用 `JobStreamData`，与 REST `/jobs/{job_id}/events` 返回的数据库审计 `JobEvent` 是两个 Schema。心跳没有 `id` 行：

```text
event: heartbeat
data: {"server_time":"2026-07-18T02:30:00Z"}

```

支持事件类型：

- `job.queued`
- `job.started`
- `job.progress`
- `job.validating`
- `job.cancel_requested`
- `job.succeeded`
- `job.failed`
- `job.cancelled`
- `stream.resync_required`
- `heartbeat`

规则：

- 持久化 Job Event 的 Event ID 为单调递增数据库 ID，不保证连续。
- 客户端重连时发送 `Last-Event-ID`。
- 持久化事件默认保留 30 天；服务端回放仍在保留期内的后续事件，然后进入实时模式。
- 若 `Last-Event-ID` 已超出保留窗口，服务端发送一次非持久化 `stream.resync_required`。其 SSE `id` 使用当前已持久化事件的高水位，不新建 Job Event；data 包含 `reason=history_expired`、`high_watermark_event_id` 和 `active_jobs_url`。前端收到后必须重新请求活动任务列表/详情，再以该高水位继续订阅。
- Heartbeat 不持久化、不携带 SSE `id`，且不改变业务状态或客户端 Last-Event-ID。
- SSE 断线不代表 Job 失败；前端必须通过 REST 对账。
- 事件不包含学习材料正文、答案、Prompt、API Key 或上游完整响应。

断档恢复示例：

```text
id: 1042
event: stream.resync_required
data: {"reason":"history_expired","high_watermark_event_id":1042,"active_jobs_url":"/api/v1/jobs?active=true"}

```

持久化事件、`stream.resync_required` 和 `heartbeat` 使用 OpenAPI 中三个不同的 SSE Envelope Schema；它们均不能伪装成 REST 审计接口使用的 `JobEvent`。

## 15. Export 与 Backup API

### 15.1 导出课时

```http
POST /api/v1/lessons/{lesson_id}/exports
```

```json
{
  "format": "markdown"
}
```

小型 Markdown 导出可同步返回 File Object；ZIP/大导出返回 Job。下载：

```http
GET /api/v1/files/{file_id}/download
```

响应设置安全的 `Content-Disposition`，文件名仅用于展示。

### 15.2 备份查询与本机命令

备份通过部署文档中的本机 Tools 命令创建，浏览器 API 不启动 `pg_dump`。API 只提供查询：

```http
GET /api/v1/backups
```

备份列表只返回状态、版本、大小和归档展示名称，不返回宿主机绝对路径，也不通过通用 File API 下载数据库备份包。

恢复数据库和文件卷不通过在线 API 执行，使用部署文档中的离线恢复脚本，避免在运行中的服务内替换自身数据库。

## 16. HTTP 状态码约定

| 状态码 | 使用场景 |
| --- | --- |
| 200 | 查询、更新、同步完成的操作 |
| 201 | 新资源创建完成 |
| 202 | 已创建后台任务 |
| 204 | 成功删除且无响应体 |
| 400 | 请求格式错误 |
| 404 | 资源不存在 |
| 409 | 版本、幂等、状态冲突 |
| 413 | 上传过大 |
| 415 | 媒体类型不支持 |
| 422 | 语义/字段校验失败 |
| 429 | 本地或上游限流映射 |
| 500 | 未预期服务端错误 |
| 503 | 必要依赖暂不可用 |
| 507 | 存储空间不足 |

## 17. API 兼容策略

- `/api/v1` 内只做向后兼容的字段新增和可选能力扩展。
- 删除/重命名字段或改变语义需要 `/api/v2` 或明确迁移窗口。
- OpenAPI 变更在 CI 中执行 `make check-openapi-breaking`；默认对比已发布快照 `api/openapi.baseline.yaml`，可用 `OPENAPI_BASELINE` 显式覆盖，快照不存在时回退到 `HEAD:api/openapi.yaml`。门禁覆盖操作、必需参数、请求体、响应媒体/Schema 与组件 Schema 的破坏性变化。
- 前端只使用生成 Client；需要自定义 SSE 包装时仍复用 OpenAPI Schema。
- 所有示例响应进入契约测试，避免文档与实现漂移。
