# Stable Error Codes

This table is for frontend integration. REST failures use Problem Details
`code`; Job failures use `job.error.code` plus the persisted per-run
`job.error.retryable`. Rows marked `job` can vary by the concrete failure cause.

## REST Problem Codes

| Code | HTTP | Retry | Meaning |
| --- | ---: | --- | --- |
| `bad_request` | 400 | no | Malformed JSON, query, multipart, or `Last-Event-ID`. |
| `not_found` | 404 | no | Resource is missing or deleted. |
| `version_conflict` | 409 | no | Optimistic lock mismatch; refresh before editing. |
| `idempotency_conflict` | 409 | no | Same idempotency key used with a different request. |
| `idempotency_in_progress` | 409 | yes | Same idempotent request is still running; honor `Retry-After`. |
| `resource_busy` | 409 | yes | An active Job blocks this action; use returned active job context if present. |
| `attempt_already_submitted` | 409 | no | Attempt answers are locked after submit. |
| `validation_error` | 422 | no | Request fields failed validation. |
| `practice_not_needed` | 422 | no | Deterministic policy says no regular practice is needed. |
| `upload_too_large` | 413 | no | Upload exceeds configured maximum. |
| `unsupported_media_type` | 415 | no | File/media type is unsupported. |
| `too_many_requests` | 429 | yes | Local rate limiting or rapid duplicate operation. |
| `claude_unavailable` | 503 | yes | Claude is temporarily unavailable. |
| `claude_not_configured` | 503 | no | Worker has no usable Claude secret; non-AI features remain usable. |
| `maintenance_mode` | 503 | yes | Consistency maintenance is active. |
| `storage_full` | 507 | no | Local data volume is out of space. |

## Job Error Codes

| Code | Retry | Frontend action |
| --- | --- | --- |
| `worker_lost` | yes | Show retry after recovery/backoff. |
| `maintenance_lease_expired` | yes | Ask user to rerun/repair maintenance flow. |
| `transcript_required` | no | Ask user to upload subtitles, paste text, or enable local transcription. |
| `material_fetch_failed` | job | Respect `job.error.retryable`; unsupported/login/tool output failures can be terminal. |
| `material_too_large` | no | Ask user to reduce file/source size or change config. |
| `material_unsupported_type` | no | Ask user to provide supported text/PDF/DOCX/subtitle media. |
| `material_parse_failed` | no | Show actionable parse failure, no raw content. |
| `url_blocked` | no | Explain URL was rejected by Worker SSRF/allowlist checks. |
| `unsupported_job_kind` | no | Developer/setup mismatch; link logs/status. |
| `job_timeout` | yes | Retry or reduce source size/work scope. |
| `publish_failed` | yes | Retry after the rolled-back publish transaction. |
| `internal_error` | job | Respect `job.error.retryable`; use request/job identifiers for local diagnostics. |
| `claude_rate_limited` | yes | Retry after waiting. |
| `claude_timeout` | yes | Retry; suggest checking network/timeout config. |
| `claude_overloaded` | yes | Retry later. |
| `claude_invalid_json` | yes | Retry; generated structure was malformed. |
| `claude_schema_invalid` | yes | Retry; generated JSON failed schema. |
| `claude_domain_invalid` | job | Respect `job.error.retryable`; missing source state is terminal, generated validation drift is retryable. |
| `claude_unavailable` | yes | Retry later and show Claude status. |
| `claude_not_configured` | no | Direct user to deployment secret setup; never ask browser to store a key. |
| `claude_auth_failed` | no | Direct user to rotate/fix the Worker secret. |
| `claude_invalid_request` | no | Developer/config issue, usually model or schema. |
| `claude_refused` | no | Show refusal as non-retryable for same input. |
| `claude_output_truncated` | no | Ask user to reduce scope or adjust Worker output config. |
