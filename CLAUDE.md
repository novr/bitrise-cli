# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Go is managed via `mise`. Prefix all `go` commands with `mise exec go@latest --`:

```bash
# Build
mise exec go@latest -- go build -o br .
make build          # equivalent shortcut

# Install to /usr/local/bin
make install

# Fetch/update dependencies
mise exec go@latest -- go mod tidy

# Compile-check without producing a binary
mise exec go@latest -- go build ./...

# Run all tests
mise exec go@latest -- go test ./...

# Run a single test
mise exec go@latest -- go test ./cmd -run TestParseStatusFilter
```

The built binary is `./br`. Run it directly: `./br build list --help`.

## Architecture

This is a `gh`-style CLI for Bitrise CI/CD. The module name is `br`.

```
cmd/                   # Cobra command definitions (one file per command group)
  root.go              # rootCmd + Execute()
  auth.go              # br auth login / logout / status
  app.go               # br app list
  build.go             # br build parent cmd + shared helpers
  build_list.go        # br build list
  build_view.go        # br build view <number>
  build_logs.go        # br build logs <number>
  logparse.go          # shared Bitrise-log step parsing (view + logs --failed-only)
  jsonout.go           # shared --json field validation + output (build/app list)
  config.go            # br config show / set-default-app
  version.go           # br version
internal/
  api/client.go        # Bitrise REST API client (https://api.bitrise.io/v0.1)
  config/config.go     # ~/.config/br/config.yml read/write
```

`Execute()` (root.go) installs a `signal.NotifyContext` and runs `ExecuteContext`; every API method takes a `context.Context` (from `cmd.Context()`) so Ctrl+C cancels in-flight requests and retry backoff.

### Key design decisions

**App slug resolution** (`cmd/build.go: resolveAppSlug`) — every build command needs a Bitrise "app slug". Resolution priority:
1. `--app` flag
2. `BITRISE_APP_SLUG` env var
3. Git remote URL matched against the user's Bitrise apps via API
4. `default_app` in `~/.config/br/config.yml`

If an `origin` remote exists but matches no accessible app, this is a hard error (not a fallback to `default_app`) to avoid silently targeting the wrong app — see the `errNoGitRemote` sentinel in `cmd/build.go`.

**Token resolution** (`internal/config/config.go: GetToken`) — `BITRISE_API_TOKEN` (or legacy `BITRISE_TOKEN`) env var beats the stored config, enabling CI/script use. Note: workspace API tokens (`bitwat_…`) cannot access `/me` or `/me/apps`, so the client uses `/apps` and validates via `Client.Verify`.

**Build status** (`internal/api: BuildStatus`) — the API's numeric status codes are a named type with constants (`StatusRunning=0 … StatusAborted=4`); never compare `build.Status` against bare ints. Status filtering is an optional `*BuildStatus` (nil = no filter), not a sentinel string.

**`--json` flag** — `--json field1,field2` outputs a subset; `--json all` (or `*`) outputs every field. Field names are camelCase (`buildNumber`, `commitMessage`, etc.) and validated in `parseJSONFields` — unknown names error out. Note: this is a normal string flag (no `NoOptDefVal`), so the field list must follow as a separate token (`--json status,branch`) — that is intentional, since an optional-value flag would swallow the space-separated field list as a positional arg.

**Log parsing** (`cmd/logparse.go: parseLogSteps`) — the single source of truth for both `build view` (step summary) and `build logs --failed-only`. It splits the raw log on Bitrise step-header lines (`| (N) step-name`) and records each section's exit code. This is best-effort and depends on the Bitrise log format staying consistent.

**Log fetching** (`internal/api/client.go: FetchLog`) — for archived (finished) builds, the log endpoint returns an `expiring_raw_log_url`; for running builds it returns `log_chunks`. `FetchLog` handles both transparently.

### Bitrise API

Base URL: `https://api.bitrise.io/v0.1`  
Auth header: `Authorization: <token>` (no "Bearer" prefix)

Key endpoints used (paths verified live — workspace tokens 404 on `/me*`):
- `GET /apps?limit=1` — token validation (`Client.Verify`); works for all token types
- `GET /me` — best-effort username for the login greeting only (404 for workspace tokens)
- `GET /apps` — list apps (for git-remote auto-detection); paginated via `paging.next`
- `GET /apps/{app-slug}/builds` — list builds; supports `build_number`, `branch`, `workflow`, `status`, `limit`
- `GET /apps/{app-slug}/builds/{build-slug}/log` — log metadata + `expiring_raw_log_url` for archived logs

Build status codes: `1=success`, `2=error` (failed), `3=aborted`, `0=running`. There is no `duration` field — derive it from `started_on_worker_at`/`finished_at` (`Build.DurationSeconds`).
