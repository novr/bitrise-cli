# br — Bitrise CLI

A `gh`-style CLI for accessing Bitrise build history and logs from your terminal or AI assistants (Claude / Cursor).

> **Unofficial tool**: This is a community CLI, not affiliated with Bitrise's official product ([bitrise-io/bitrise](https://github.com/bitrise-io/bitrise)). Bitrise is a trademark of Bitrise Ltd. It uses the Bitrise API v0.1; behavior is not guaranteed.

## Installation

```bash
brew install novr/taps/br          # macOS (Apple Silicon / Intel)
```

On Linux, download from [Releases](https://github.com/novr/bitrise-cli/releases) (`br_<version>_linux_<arch>.tar.gz`):

```bash
VERSION=0.1.0
ARCH=$(uname -m)   # x86_64 → amd64, aarch64 → arm64
case "${ARCH}" in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac
curl -fsSL "https://github.com/novr/bitrise-cli/releases/download/v${VERSION}/br_${VERSION}_linux_${ARCH}.tar.gz" | tar -xz
sudo mv br /usr/local/bin/
```

Or:

```bash
go install github.com/novr/bitrise-cli/cmd/br@latest
```

Or build / install manually:

```bash
make install                 # installs to /usr/local/bin/br
go build -o br ./cmd/br      # builds br in the current directory
```

## Authentication

Create a Bitrise [Personal Access Token](https://app.bitrise.io/me/profile#/security), then:

```bash
br auth login
# Create a Personal Access Token at https://app.bitrise.io/me/profile#/security
# ? Paste your Bitrise Personal Access Token: ********************
# ✓ Logged in
```

The login flow prints the token URL only; it does not open a browser. For CI and scripts, pipe the token on stdin:

```bash
echo "$BITRISE_API_TOKEN" | br auth login --with-token
```

Environment variables take precedence (no `br auth login` required). Use `BITRISE_API_TOKEN` (recommended) or `BITRISE_TOKEN`:

```bash
export BITRISE_API_TOKEN=<your-token>
```

## Usage

### List builds

The Bitrise app is resolved from `--app`, `BITRISE_APP_SLUG`, `.br.yml`, or the git remote (see [App auto-detection](#app-auto-detection)).

```bash
br build list
br build list --limit 20
br build list --branch main --status failed   # status: success/failed/error/running/aborted
```

> If a git remote exists but matches no accessible Bitrise app, the command errors (to avoid targeting the wrong app). Override with `--app <slug>` or `.br.yml`.

**JSON output for AI assistants (Claude / Cursor):**

```bash
br build list --limit 3 --json status,buildNumber,branch,workflow
```

```json
[
  {"status": "success", "buildNumber": 124, "branch": "main", "workflow": "primary"},
  {"status": "failed",  "buildNumber": 123, "branch": "feature/auth", "workflow": "deploy"},
  {"status": "running", "buildNumber": 122, "branch": "main", "workflow": "primary"}
]
```

`--json all` returns every field. Unknown field names produce an error.

### Build details

```bash
br build view 123
# ✗ failed  #123  deploy  (branch: feature/auth)
#   Commit:    add-login  (abc1234)
#   Triggered: 15m ago
#   Duration:  5m32s
#
#   ✗ Step failed: run-xcode-tests@2.4.1 (exit code: 1)
#
#   To see full logs:   br build logs 123
#   To see errors only: br build logs 123 --failed-only
```

### Logs

```bash
br build logs 123               # full log
br build logs 123 --failed-only # failed steps only
```

`--failed-only` is especially useful when asking Claude / Cursor to analyze logs and suggest fixes.

### App list

```bash
br app list
br app list --json slug,title   # fields: slug, title, repoURL, or all
```

### Version

```bash
br version
```

### Configuration

```bash
br config show                # global config + effective .br.yml path/app
br config set app <app-slug>  # write .br.yml in the current directory
```

### Diagnostics

```bash
br doctor   # check auth, app resolution, and API reachability (CI-friendly)
```

## Common flags

| Flag | Description |
|------|-------------|
| `--app <slug>` | Bitrise app slug (overrides auto-detection) |
| `BITRISE_APP_SLUG` | App slug via environment variable |

## App auto-detection

`br build` commands resolve the app in this order:

1. `--app` flag
2. `BITRISE_APP_SLUG` environment variable
3. `.br.yml` (current directory upward to git root)
4. `git remote get-url origin` matched against Bitrise app `repo_url`

Global `default_app` was removed. A single home-dir fallback could silently target the wrong app in monorepos and multi-repo setups.

### Project-local config (`.br.yml`)

Commit app slugs to the repo so the team shares the same target. Discovery walks up to git root (not `bitrise.yml`), because Bitrise monorepos usually centralize CI at the repo root and subpackages should inherit the root `.br.yml`.

```yaml
app: <app-slug>
```

```
monorepo/
  .br.yml          # used when a subdirectory has no .br.yml
  ios/
    .br.yml        # point at a different Bitrise app on the same origin
```

Run `br config set app <slug>` to write the file in the current directory. Use `--app` when the fork's origin does not match Bitrise. Slug/git mismatches are easy to miss in daily use; run `br doctor` in CI.

## AI assistant workflow

Ask Claude or Cursor something like:

> Check whether the latest Bitrise build failed; if it did, analyze the logs and suggest fixes.

```bash
br build list --limit 1 --json status,buildNumber
br build logs 123 --failed-only
```

## Config files

`~/.config/br/config.yml` stores the token only. App slugs live in project `.br.yml` (commit recommended for team sharing and monorepo switching).

```yaml
token: <your-token>
```

> **Security note**: Tokens are stored in plain text with mode `0600` (owner read/write only). Like `gh`, there is no OS keychain encryption. On shared machines, prefer the `BITRISE_API_TOKEN` environment variable.
