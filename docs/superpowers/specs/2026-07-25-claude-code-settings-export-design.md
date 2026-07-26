# Claude Code Settings Export Design

**Date:** 2026-07-25

**Status:** Approved

## Goal

Allow the user to select a saved Claude gateway Key in Settings and overwrite
the host Claude Code configuration at `~/.claude/settings.json` with that Key.

## Scope

- Add one explicit, confirmed "save to Claude Code" action for each saved Key.
- Keep the complete Key server-side: the browser sends only the Key ID and the
  API reads the secret from `keys.json`.
- Write exactly the requested Claude Code `env` values, with
  `ANTHROPIC_AUTH_TOKEN` set to the selected saved Key.
- Use an atomic same-directory write and `0600` file mode.
- Mount the host Claude Code directory into the API container only. Worker and
  Web must not receive this mount.

## Non-Goals

- Do not return, display, download, log, or persist the full Key in browser
  state.
- Do not automatically export on every key create/edit; exporting is a
  separate explicit action to avoid selecting the wrong Key in a multi-Key
  deployment.
- Do not change Worker routing or legacy key fallback behavior.

## API

Add this same-origin, write-gated endpoint:

```http
POST /api/v1/system/claude-keys/{key_id}/claude-code-settings
```

The response is a non-sensitive `200` JSON object:

```json
{
  "status": "saved",
  "target": "~/.claude/settings.json"
}
```

The endpoint reads the requested saved Key from the existing gateway store. It
returns the existing sanitized not-found problem if the Key does not exist and
uses a stable, secret-free `claude_code_settings_unavailable` problem for a
missing or unwritable host mount. It requires
`CLAUDE_GATEWAY_WRITE_ENABLED=true`.

## File Writer

`ClaudeCodeSettingsWriter` is a narrow handler dependency. It accepts a Key
only in process memory, serializes this exact logical document, and delegates
to the existing atomic-file helper:

```json
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_BASE_URL": "https://app.haoshuai.cc.cd",
    "ANTHROPIC_AUTH_TOKEN": "<selected saved key>",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}
```

The only on-disk full Key copy is the intended host target. Errors and API
responses must never include the serialized document, file content, or Key.

## Frontend

Place an icon-and-text action beside each masked Key in the existing settings
card. The action opens a confirmation dialog naming the selected Key and the
target `~/.claude/settings.json`; confirmation invokes the new endpoint and
shows only its safe success/error state. The Key value remains absent from the
query cache, dialog state, fixtures, and snapshots.

## Deployment Boundary

`deploy/scripts/init.sh` records the invoking user's absolute
`~/.claude` path in the ignored deployment `.env` when
`CLAUDE_CODE_SETTINGS_DIR` has not been configured, and creates the directory
when it is absent. `deploy/compose.yaml` mounts that exact directory to
`/claude-code` in API alone and passes
`CLAUDE_CODE_SETTINGS_FILE=/claude-code/settings.json`. The config verifier
asserts API has the mount and Worker/Web do not.

Because this intentionally grants the local API process host Claude Code
configuration write access, it is limited to this single directory and remains
behind the local Settings confirmation and existing write-enabled gate.

## Verification

- Unit/integration test: selected Key writes the exact document, mode `0600`,
  and an atomic replacement while the HTTP response stays secret-free.
- Integration test: missing Key, disabled write mode, and unavailable target
  return safe errors without writing a file.
- Frontend test: confirmation calls the selected endpoint and never renders a
  synthetic full Key.
- `make generate-openapi`, backend/Frontend checks, and deployment boundary
  verification all pass.
