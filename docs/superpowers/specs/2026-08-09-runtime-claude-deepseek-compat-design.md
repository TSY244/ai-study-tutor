# Runtime Claude Code Configuration And DeepSeek Compatibility Design

**Date:** 2026-08-09

**Status:** Approved

## Goal

Make the Claude Code configuration owned and consumed by the environment in
which the application runs, and make the structured Anthropic-compatible
request path work with DeepSeek V4.

- A Docker deployment writes the Worker container user's real
  `~/.claude/settings.json`; it must not overwrite the deployment host user's
  Claude Code configuration.
- A native deployment writes the native runtime user's
  `~/.claude/settings.json`.
- Worker Claude Code subprocesses consume the selected runtime configuration.
- Direct structured requests to DeepSeek V4 remain deterministic and do not
  fail because DeepSeek enables thinking mode by default.
- Connectivity-test failures leave the Key in a terminal diagnostic state and
  show an accurate message.

## Confirmed Root Causes

1. The existing Docker deployment bind-mounts the invoking host user's
   `~/.claude` directory into API only. The save action therefore intentionally
   overwrites the host file, while Worker cannot access the result.
2. Worker Claude Code subprocesses set `HOME` to `APP_DATA_DIR` and inject a
   Key selected from `keys.json`. The saved host settings file is not part of
   this execution path.
3. DeepSeek V4 enables thinking mode by default. The application sends a
   forced Anthropic tool choice for `structured_result`; DeepSeek rejects that
   combination with HTTP 400 and `Thinking mode does not support this
   tool_choice`.
4. The frontend presents every `claude_invalid_request` as a Web Fetch gateway
   failure, even for a `claude_connectivity_test`.
5. A targeted connectivity test marks a Key as `testing`, but a non-failover
   error updates only diagnostic fields. The Key therefore remains visually
   stuck in `testing` after the Job has failed.

## Runtime Ownership

### Docker

The Worker process will have an explicit, writable, persistent home:

```text
HOME=/var/lib/ai-study-tutor
~/.claude/settings.json
  = /var/lib/ai-study-tutor/.claude/settings.json
```

The existing `app_data` volume is already mounted into both API and Worker.
API writes the settings file into that volume, and Worker runs `claude` with
the same `HOME`. The Claude Code CLI therefore discovers the file through its
normal user-settings lookup.

Docker initialization will no longer capture or mount the deployment host's
`~/.claude` directory. `CLAUDE_CODE_SETTINGS_DIR` and the `/claude-code` bind
mount are removed. Compose explicitly sets the runtime `HOME` and
`CLAUDE_CODE_SETTINGS_FILE` for the relevant services so the boundary is
auditable.

### Native execution

When `CLAUDE_CODE_SETTINGS_FILE` is not explicitly set, API resolves the
target from the native process `HOME`:

```text
$HOME/.claude/settings.json
```

Worker Claude Code subprocesses retain the native runtime `HOME`. An explicit
`CLAUDE_CODE_SETTINGS_FILE` remains available for operators who need a custom
runtime home or service account.

An empty or unavailable runtime home makes the save capability unavailable
with the existing sanitized service error; it must never fall back to an
unrelated host path.

## Applying A Key To Runtime Claude Code

The existing safe endpoint remains ID-based and never returns the full Key.
Its user-facing action changes from "save to local Claude Code" to "apply to
runtime Claude Code".

The settings document contains the selected Key's Base URL, token, and model:

```json
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.deepseek.com/anthropic",
    "ANTHROPIC_AUTH_TOKEN": "<selected saved key>",
    "ANTHROPIC_MODEL": "deepseek-v4-flash",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "deepseek-v4-flash",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "deepseek-v4-flash",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "deepseek-v4-flash",
    "CLAUDE_CODE_SUBAGENT_MODEL": "deepseek-v4-flash",
    "AI_STUDY_TUTOR_CLAUDE_KEY_ID": "ck_example",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}
```

The writer keeps the existing atomic replacement and `0600` mode. It creates
`~/.claude` with mode `0700` when necessary.

The file becomes authoritative for choosing the Key used by Worker Claude Code
tasks. `AI_STUDY_TUTOR_CLAUDE_KEY_ID` is a non-secret application selector;
Claude Code simply ignores it as an unrelated environment value. A shared
resolver reads that ID and loads the exact saved Key. It then verifies that the
settings token, normalized Base URL, and model still match the selected record,
without logging any of those values. This prevents duplicate tokens saved with
different models or Base URLs from resolving to the wrong record.

If the settings file is absent, or it is a pre-existing user Claude settings
file without `AI_STUDY_TUTOR_CLAUDE_KEY_ID`, the resolver falls back to the
existing priority/weight rule. This preserves existing deployments before the
explicit apply action is used. Once an application selector is present, the
selection is authoritative: an unreadable or malformed document, an unknown or
disabled ID, or a token/Base URL/model mismatch produces a sanitized
configuration error instructing the user to apply a Key again. It never
silently switches an explicitly configured runtime to another Key.

The subprocess environment continues to contain a minimal allowlist. It
injects the resolved Key, Base URL, and model variables so that execution and
AI-run metadata cannot drift from the runtime settings selection. Claude Code
also sees the same values in its normal settings file.

The URL extractor and quiz generator use the same resolver. No host Claude
Code process is invoked by a Docker deployment.

## DeepSeek Structured Request Compatibility

Normal provider requests use a forced `structured_result` client tool because
the application requires machine-validated JSON. Removing `tool_choice` would
allow DeepSeek's default thinking mode, but it would weaken that deterministic
contract.

For models whose normalized name starts with `deepseek-`, the Anthropic request
adds:

```json
"thinking": {"type": "disabled"}
```

and retains the forced tool choice. This exact combination has been verified
against the configured official DeepSeek Anthropic endpoint. Native Anthropic
models omit the field, preserving their existing request contract. URL and
quiz Claude Code subprocesses are not changed to direct API Web Fetch; Claude
Code manages its own agent/tool protocol.

## Error And State Presentation

`claude_invalid_request` becomes a generic, actionable compatibility message:
check model name, Base URL, and provider request compatibility. It no longer
claims Web Fetch is the cause. URL-specific extraction errors continue to use
`material_fetch_failed`.

A targeted connectivity-test non-failover failure transitions the Key from
`testing` to `unknown` while preserving `last_failure_at`,
`last_error_code`, and latency. Authentication and transient failures retain
their existing `auth_failed` and cooling behavior. A successful test still
sets `healthy`.

Frontend labels, confirmation text, success feedback, OpenAPI summaries, and
deployment documentation use "运行服务" or "运行环境" rather than "本机".

## Security Boundaries

- Browser state and API responses never contain the full Key.
- Web never receives a Claude configuration mount or environment variable.
- API can write only the configured runtime settings file and the existing
  gateway configuration.
- Worker receives read/write access to its persistent home because Claude Code
  needs normal runtime state there; it does not receive gateway write APIs.
- Docker no longer receives any host `~/.claude` bind mount.
- Logs and Job errors remain sanitized and never include provider response
  bodies, settings documents, source URLs, or tokens.

## Verification

1. Backend writer tests assert runtime target text, model variables, atomic
   replacement, directory/file modes, and secret-free responses.
2. Runtime resolver tests cover selected enabled Key, missing settings,
   priority fallback, malformed/unreadable files, unknown/disabled selector
   IDs, token/Base URL/model mismatches, duplicate tokens, and secret-free
   errors.
3. URL and quiz CLI tests assert the resolved Key/Base URL/model environment
   and runtime `HOME`.
4. Anthropic provider tests assert DeepSeek requests include disabled thinking
   with forced tool choice, while Claude requests omit `thinking`.
5. State tests assert a failed targeted test exits `testing` without entering
   cooldown.
6. Frontend tests assert runtime wording and the corrected invalid-request
   message.
7. Deployment verification asserts API and Worker share the runtime home,
   Web has no Claude configuration, and no host Claude settings mount remains.
8. After rebuilding Docker, apply the existing DeepSeek Key, verify the file
   exists at the Worker container's `$HOME/.claude/settings.json`, run a real
   DeepSeek connectivity Job, and confirm it succeeds and the Key becomes
   healthy.
