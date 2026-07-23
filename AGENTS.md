# AGENTS.md

Guidance for AI coding agents working in this repository.

## Commands

Go is managed via `mise`. Prefix all `go` commands with `mise exec go@latest --`:

```bash
mise exec go@latest -- go build -o br ./cmd/br
make build
make install
mise exec go@latest -- go mod tidy
mise exec go@latest -- go build ./...
mise exec go@latest -- go test ./...
mise exec go@latest -- go test ./cmd -run TestParseStatusFilter
```

The built binary is `./br`.

## Architecture

`gh`-style CLI for Bitrise CI/CD. Module: `github.com/novr/bitrise-cli`, binary: `br`.

```
cmd/br/main.go
cmd/                   # Cobra commands
internal/api/          # Bitrise REST client
internal/config/       # global config + .br.yml discovery
```

`Execute()` installs `signal.NotifyContext` so Ctrl+C cancels in-flight API calls.

### Key design decisions

**App slug resolution** (`cmd/build.go: resolveAppSlug`) — Priority: `--app` → `BITRISE_APP_SLUG` → `.br.yml` (cwd → git root) → git `origin` matched via API. Global `default_app` was removed: a single home-dir fallback silently targeted the wrong app in multi-repo and monorepo setups. `.br.yml` is per-directory and committable so each package can pin its own slug.

Git root caps `.br.yml` walk (not `bitrise.yml`) because Bitrise monorepos typically keep CI config at the repo root; a nearer `bitrise.yml` would stop discovery before the shared root `.br.yml`. An `origin` that matches no accessible app is a hard error — falling through would hit the wrong slug. `errNoGitRemote` marks benign git absence only. Build commands authenticate before resolution because they always call the API afterward.

**Local config** (`internal/config/local.go`) — `app` only; empty/whitespace skips to parent. Outside git, only cwd is checked so parent directories cannot leak in. `br doctor` probes git even when `.br.yml` wins, to warn on slug/git mismatch without affecting normal commands.

**Token resolution** (`internal/config/config.go: GetToken`) — Env vars beat stored config for CI/scripts. Workspace tokens (`bitwat_…`) 404 on `/me*`, so validation uses `GET /apps?limit=1` (`Client.Verify`).

**Build status** (`internal/api: BuildStatus`) — Named constants (`StatusRunning=0 … StatusAborted=3`); never compare bare ints. Filters use `*BuildStatus` (nil = no filter).

**`--json` flag** — Normal string flag (no `NoOptDefVal`) so the field list stays a separate token; an optional-value flag would swallow `status,branch` as a positional arg. `build view --json` emits a single object and adds `failedSteps`; log is fetched only when that field is requested (or with `all`).

**Log parsing** (`cmd/logparse.go: parseLogSteps`) — Shared by `build view` and `build logs --failed-only` so step summaries and filtered logs never disagree. Best-effort; depends on Bitrise log format.

**Log fetching** (`internal/api/client.go: FetchLog`) — Archived builds expose `expiring_raw_log_url`; running builds stream `log_chunks`. Both paths must work for AI assistants polling in-progress builds.

**`build watch`** (`cmd/build_watch.go`) — Polls `GetBuildByNumber` until status is no longer running. `--exit-status` returns an error (exit 1) on failure/aborted; default is false to match other read commands. Minimum `--interval` is 3s. With `--json`, poll output is discarded so stdout contains only the final JSON object.

**`build list --branch @current`** (`cmd/build.go: currentGitBranch`) — Resolves via `git rev-parse --abbrev-ref HEAD` from the process cwd (git walks up to the repo root). Detached HEAD (`HEAD`) and non-repo directories error before auth/API. Whitespace around `@current` is trimmed.

**`build logs --json`** (`cmd/build_logs.go`) — Structured log output with `steps` (`[{name, exitCode}]`) and `failedStepLogs` (`[{name, exitCode, body}]`). Mutually exclusive with `--failed-only`. `failedSteps` on view/watch is name-only summary; use `failedStepLogs` for raw step bodies. Parse failure: empty arrays + stderr (same message as `--failed-only`).

**CI** — PRs run `go test` and actionlint. actionlint does not validate reusable-workflow inputs in external repos; verify release workflows with `workflow_dispatch` before tagging.

### Bitrise API

Base URL: `https://api.bitrise.io/v0.1`  
Auth: `Authorization: <token>` (no `Bearer`)

Endpoints (workspace tokens 404 on `/me*`):
- `GET /apps?limit=1` — `Client.Verify`; works for all token types
- `GET /me` — login greeting only; 404 for workspace tokens
- `GET /apps` — git-remote detection; paginated via `paging.next`
- `GET /apps/{app-slug}/builds` — `build_number`, `branch`, `workflow`, `status`, `limit`
- `GET /apps/{app-slug}/builds/{build-slug}/log` — archived log URL or chunks

Status codes: `1=success`, `2=error`, `3=aborted`, `0=running`. No `duration` field — use `Build.DurationSeconds`.
