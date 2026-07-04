# br — Bitrise CLI

`gh` ライクな操作感で Bitrise のビルド履歴・ログにターミナルや AI アシスタント（Claude / Cursor）からシームレスにアクセスできる CLI ツール。

## インストール

```bash
make install   # /usr/local/bin/br にインストール
```

または手動ビルド:

```bash
mise exec go@latest -- go build -o br .
```

## 認証

Bitrise の [Personal Access Token](https://app.bitrise.io/me/profile#/security) を取得して:

```bash
br auth login
# Opening https://app.bitrise.io/me/profile#/security ... (端末の場合のみ)
# ? Paste your Bitrise Personal Access Token: ********************
# ✓ Logged in as your_username (your@email.com)
```

ブラウザを開きたくない場合は `--no-browser`。CI・スクリプトでは標準入力からトークンを渡せます:

```bash
br auth login --no-browser
echo "$BITRISE_TOKEN" | br auth login --with-token
```

環境変数を使う場合は最優先で参照されます（`br auth login` 不要）:

```bash
export BITRISE_TOKEN=<your-token>
```

## 使い方

### ビルド一覧

カレントディレクトリの git remote から Bitrise アプリを自動検出します。

```bash
br build list
br build list --limit 20
br build list --branch main --status failed   # status: success/failed/error/running/aborted
```

> git remote があるのに対応する Bitrise アプリが見つからない場合は、`default_app` にフォールバックせずエラーになります（誤ったアプリを参照しないため）。`--app <slug>` で明示できます。

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
br config show                       # 設定ファイルの場所・認証状態・default_app を表示
br config set-default-app <app-slug> # git 検出できない時に使うアプリを設定
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
3. `git remote get-url origin` → Bitrise アプリの `repo_url` と照合
4. `~/.config/br/config.yml` の `default_app`

## AI アシスタントとの連携例

Claude や Cursor に次のように依頼するだけで、裏側でこの CLI が実行されます:

> 「直近の Bitrise ビルドが落ちてないか確認して、落ちてたらログを解析して修正案を出して」

```bash
# Claude/Cursor が実行するコマンドの流れ
br build list --limit 1 --json status,buildNumber
br build logs 123 --failed-only
```

## 設定ファイル

`~/.config/br/config.yml` にトークンとデフォルトアプリが保存されます。

```yaml
token: <your-token>
default_app: <app-slug>  # オプション
```

> **セキュリティ注記**: トークンはパーミッション `0600`（本人のみ読み書き可）の平文で保存されます。`gh` などと同様、OS キーチェーンによる暗号化は行いません。共有マシンでは環境変数 `BITRISE_TOKEN` の利用を推奨します。
