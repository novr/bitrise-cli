# br — Bitrise CLI

`gh` ライクな操作感で Bitrise のビルド履歴・ログにターミナルや AI アシスタント（Claude / Cursor）からシームレスにアクセスできる CLI ツール。

> **非公式ツール**: これは個人が開発した非公式の CLI で、Bitrise 社の公式製品（[bitrise-io/bitrise](https://github.com/bitrise-io/bitrise)）とは無関係です。Bitrise は Bitrise 社の商標です。Bitrise API v0.1 を利用しますが、動作は保証されません。

## インストール

```bash
brew install novr/taps/br          # macOS (Apple Silicon / Intel)
```

Linux は [Releases](https://github.com/novr/bitrise-cli/releases) から取得（`br_<version>_linux_<arch>.tar.gz`）:

```bash
VERSION=0.1.0
ARCH=$(uname -m)   # x86_64 → amd64, aarch64 → arm64
case "${ARCH}" in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac
curl -fsSL "https://github.com/novr/bitrise-cli/releases/download/v${VERSION}/br_${VERSION}_linux_${ARCH}.tar.gz" | tar -xz
sudo mv br /usr/local/bin/
```

または:

```bash
go install github.com/novr/bitrise-cli/cmd/br@latest   # br バイナリを $GOBIN に
```

または手動ビルド / インストール:

```bash
make install                 # /usr/local/bin/br にインストール
go build -o br ./cmd/br      # カレントに br をビルド
```

## 認証

Bitrise の [Personal Access Token](https://app.bitrise.io/me/profile#/security) を取得して:

```bash
br auth login
# Create a Personal Access Token at https://app.bitrise.io/me/profile#/security
# ? Paste your Bitrise Personal Access Token: ********************
# ✓ Logged in
```

トークン発行ページは URL 表示のみで、ブラウザは自動では開きません。CI・スクリプトでは標準入力からトークンを渡せます:

```bash
echo "$BITRISE_API_TOKEN" | br auth login --with-token
```

環境変数を使う場合は最優先で参照されます（`br auth login` 不要）。`BITRISE_API_TOKEN`（推奨）または `BITRISE_TOKEN`:

```bash
export BITRISE_API_TOKEN=<your-token>
```

## 使い方

### ビルド一覧

カレントディレクトリの git remote から Bitrise アプリを自動検出します。

```bash
br build list
br build list --limit 20
br build list --branch main --status failed   # status: success/failed/error/running/aborted
```

> git remote があるのに対応する Bitrise アプリが見つからない場合はエラーになります（誤ったアプリを参照しないため）。`--app <slug>` や `.br.yml` で明示できます。

**AI（Claude / Cursor）向け JSON 出力:**

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

`--json all` で全フィールドを返します。未知のフィールド名を指定するとエラーになります。

### ビルド詳細

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

### ログ確認

```bash
br build logs 123               # フルログを出力
br build logs 123 --failed-only # 失敗したステップのログだけ出力
```

`--failed-only` は Claude / Cursor に「このログを元にコードを修正して」と渡す際に特に有効です。

### アプリ一覧

```bash
br app list
br app list --json slug,title   # フィールド: slug, title, repoURL, または all
```

### バージョン

```bash
br version
```

### 設定

```bash
br config show                # グローバル設定 + 有効な .br.yml のパス/app を表示
br config set app <app-slug>  # カレントディレクトリに .br.yml を書き込み
```

### 診断

```bash
br doctor   # 認証・app 解決・API 到達性を確認（CI 向け）
```

## フラグ共通オプション

| フラグ | 説明 |
|--------|------|
| `--app <slug>` | Bitrise アプリスラグを直接指定（自動検出を上書き） |
| `BITRISE_APP_SLUG` | 環境変数でアプリを指定 |

## アプリの自動検出

`br build` 系コマンドは以下の優先順位でアプリを特定します:

1. `--app` フラグ
2. `BITRISE_APP_SLUG` 環境変数
3. `.br.yml`（カレントディレクトリから親方向、git root まで）
4. `git remote get-url origin` → Bitrise アプリの `repo_url` と照合

### プロジェクトローカル設定（`.br.yml`）

リポジトリにコミットするプロジェクト設定です。モノレポではサブディレクトリごとに置けます。

```yaml
app: <app-slug>
```

```
monorepo/
  .br.yml          # 共通 app（サブに無ければ継承）
  ios/
    .br.yml        # ios 専用 app
```

`br config set app <slug>` でカレントディレクトリに書き込めます。fork 先など origin が Bitrise と一致しない場合は `--app` を使ってください。

## AI アシスタントとの連携例

Claude や Cursor に次のように依頼するだけで、裏側でこの CLI が実行されます:

> 「直近の Bitrise ビルドが落ちてないか確認して、落ちてたらログを解析して修正案を出して」

```bash
# Claude/Cursor が実行するコマンドの流れ
br build list --limit 1 --json status,buildNumber
br build logs 123 --failed-only
```

## 設定ファイル

`~/.config/br/config.yml` にトークンが保存されます。アプリ slug はプロジェクトの `.br.yml` に置きます（コミット推奨）。

```yaml
token: <your-token>
```

> **セキュリティ注記**: トークンはパーミッション `0600`（本人のみ読み書き可）の平文で保存されます。`gh` などと同様、OS キーチェーンによる暗号化は行いません。共有マシンでは環境変数 `BITRISE_API_TOKEN` の利用を推奨します。
