# 06. エージェント機能 Phase 3 実装仕様書 (Code Search Specification)
本ドキュメントは `docs/03_agent_update.md` v2.0.0 に定義された Phase 3 のうち、`search_code` を実装するための具体仕様書である。
Phase 1 (`docs/04_agent_specification.md`) および Phase 2 (`docs/05_agent_specification_2.md`) が完了していることを前提とする。
本フェーズでは project index file の生成は行わず、コード検索機能にスコープを限定する。
**ステータス**: 未着手
---
## 用語定義
| 用語 | 意味 |
|------|------|
| **search_code** | ワークスペース内の全ファイルを対象に正規表現パターンでコード検索する Tool |
| **SearchResult** | `search_code` の検索結果を表す構造体（ファイルパス、行番号、マッチ行の内容） |
| **file_pattern** | 検索対象のファイル名を絞り込むための glob パターン。`*.js`, `*.css` などの basename ベースを想定する |
---
## Phase 3 の境界
### In Scope
- `search_code` Tool の実装
- `artifacts.Manager.SearchFiles` メソッドの追加
- System Prompt の Phase 3 更新
- `search_code` に関するテスト追加

### Out of Scope
- `generate_index` Tool
- `artifacts.CodeIndexer` と project index file の生成
- セマンティック検索（ベクトル DB）
- 複数ファイル同時編集の最適化
- 並列 Tool 呼び出し
- マルチエージェントアーキテクチャ
- 行範囲付き `read_file`（将来検討）

### 前提条件
- Phase 1 + Phase 2 主経路が完了していること
- 以下が実装済みであること:
  - `read_file`, `write_file`, `list_files`, `apply_edit` Tool
  - `provider.Provider.StreamWithTools`（Ollama / OpenRouter）
  - `ChatService.sendMessageWithTools` による Tool Call ループ
  - Tool 結果の切り詰め (50KB) と古い Tool 結果の要約置換

### 受け入れ条件
- `search_code` により正規表現パターンでワークスペース内の全ファイルを検索できる
- `search_code` で `file_pattern` によるファイル種別フィルタリングが可能
- マッチ結果がファイルパス・行番号つきで返る
- Phase 1/2 の既存 Tool が従来通り動作する
- Markdown モードが影響なく動作する
- Phase 3 の実装は workspace に追加の内部管理ファイルを生成しない
---
## 変更ファイル一覧
```text
新規:
  artifacts/search.go             # SearchResult 型, Manager.SearchFiles メソッド
  artifacts/search_test.go        # SearchFiles のテスト
  tools/search_code_tool.go       # search_code Tool
  tools/search_code_tool_test.go  # search_code Tool のテスト

変更:
  chat.go
    ... search_code の Tool 登録追加
    ... buildSystemPrompt の Phase 3 更新 (search_code 追加)
    ... buildToolDefinitions に search_code を含める
```
---
## 推奨実装順序
1. `artifacts/search.go` — SearchFiles の純粋ロジック
2. `tools/search_code_tool.go` — search_code Tool
3. `chat.go` — Tool 登録、System Prompt 更新
4. 結合確認、手動確認

各ステップは **実装 → パッケージ単体テスト → 次ステップ** の順で進める。
---
## Step 1: `artifacts/search.go`
### 変更内容
```go
package artifacts

import (
    "bufio"
    "os"
    "path/filepath"
    "regexp"
)

const maxSearchResults = 50
const maxSearchFileSize = 1 * 1024 * 1024

type SearchResult struct {
    File    string
    Line    int
    Content string
}

func (m *Manager) SearchFiles(sessionID, pattern string, filePattern string) ([]SearchResult, error)
```

### SearchFiles 処理フロー
1. `pattern` を `regexp.Compile` でコンパイル（不正な正規表現はエラー）
2. `filePattern` が空でなければ `filepath.Match` 用のパターンとして保持
3. `filepath.Walk` でワークスペースディレクトリを走査
4. 各ファイルについて:
   a. ディレクトリはスキップ
   b. シンボリックリンク経由の workspace 外参照を拒否（`filepath.EvalSymlinks` で検証）
   c. `filePattern` によるファイル名フィルタリング
   d. ファイルサイズ > 1MB はスキップ
   e. バイナリファイル判定（先頭 512 バイトに `\x00` を含む場合はスキップ）
   f. 1 行ずつ正規表現マッチング
   g. マッチした行を `SearchResult` に追加
5. 結果が `maxSearchResults` (50) に達したら走査を打ち切り
6. 結果を返す

### 要件
- `SearchResult.File` はワークスペースからの相対パスとする
- `SearchResult.Line` は 1-origin の行番号
- `SearchResult.Content` はマッチした行の内容（前後の空白はそのまま）
- `filePattern` は `filepath.Match` 互換の glob パターン（例: `*.js`, `*.css`）。パス区切りを含むパターンは受け付けない
- `filePattern` が空文字の場合は全ファイルを対象とする
- パストラバーサル攻撃を防ぐため、ワークスペース外のファイルは走査しない
- シンボリックリンク経由の workspace 外参照は `ListFiles` と同等の `filepath.EvalSymlinks` で拒否する
- バイナリファイルは検索対象外とする
- 結果順は walk 順に依存させず、少なくとも `file path` → `line number` の昇順で安定化させる

### テスト
- 基本検索: パターンにマッチする行が結果に含まれる
- 複数ファイルにまたがる検索結果
- `filePattern` によるフィルタリング（`*.js` のみマッチ、`*.css` は除外）
- 不正な正規表現パターン → エラー
- マッチなし → 空の結果（エラーではない）
- バイナリファイル → スキップされる
- 大きなファイル (>1MB) → スキップされる
- 結果数上限 (50) → 50 件で打ち切られる
- 空のワークスペース → 空の結果
- サブディレクトリ内のファイルも検索される
---
## Step 2: `tools/search_code_tool.go`
### 変更内容
```go
package tools

type SearchCodeTool struct {
    manager *artifacts.Manager
}

func NewSearchCodeTool(m *artifacts.Manager) *SearchCodeTool
func (t *SearchCodeTool) Name() string        // "search_code"
func (t *SearchCodeTool) Description() string
func (t *SearchCodeTool) Parameters() map[string]any
func (t *SearchCodeTool) Execute(sessionID string, args map[string]any) (string, error)
```

### Parameters スキーマ
```json
{
  "name": "search_code",
  "description": "Search for a pattern in all files in the artifact workspace. Returns matching files and line numbers.",
  "parameters": {
    "type": "object",
    "properties": {
      "pattern": {
        "type": "string",
        "description": "The regex pattern to search for."
      },
      "file_pattern": {
        "type": "string",
        "description": "Optional file pattern to filter (e.g., '*.ts', '*.go')."
      }
    },
    "required": ["pattern"]
  }
}
```

### Execute 処理フロー
1. `pattern` を args から取得（不足時はエラー）
2. `file_pattern` を args から取得（省略可、デフォルト空文字）
3. 各引数の型が `string` であることを検証
4. `manager.SearchFiles(sessionID, pattern, filePattern)` を呼び出し
5. 結果をフォーマットして返す:
   - マッチあり:
     ```
     Found 3 matches in 2 files:

     path/to/file.ext:15:  matching line content
     other/file.js:7:      another match
     other/file.js:42:     yet another match
     ```
   - マッチなし: `"No matches found for pattern: <pattern>"`

### 要件
- フォーマットは grep ライクな `ファイルパス:行番号:  内容` 形式とする
- ヘッダ行に総マッチ数とファイル数を含める
- LLM がファイルと行を特定しやすい形式にする
- 引数型不正（例: `pattern: 123`）はエラーにする
- 検索結果が多い場合でも、`SearchFiles` の上限を超える件数を返さない

### テスト
- 正常系: マッチあり → ヘッダ行 + フォーマット済み行
- 正常系: マッチなし → `"No matches found"` メッセージ
- 引数不足（pattern）→ エラー
- 引数型不正 → エラー
- `file_pattern` なしでも動作する
---
## Step 3: `chat.go`
### 3.1 Tool 登録の追加
`NewChatService` に `search_code` を登録する。

```go
tm := tools.NewToolManager()
tm.Register(tools.NewReadFileTool(artifactMgr))
tm.Register(tools.NewWriteFileTool(artifactMgr))
tm.Register(tools.NewListFilesTool(artifactMgr))
tm.Register(tools.NewApplyEditTool(artifactMgr))
tm.Register(tools.NewSearchCodeTool(artifactMgr))
```

### 3.2 System Prompt の Phase 3 更新
Agent モードの System Prompt に `search_code` の説明を追加する。
既存の Phase 2 prompt は維持しつつ、大きなプロジェクトや横断調査時の指示として以下を追記する:

```
When working across multiple files or investigating an existing project:
1. Use list_files to understand the file layout
2. Use search_code to find relevant code patterns across files
3. Use read_file to inspect the specific files you need before making changes
```

System Prompt 内の `Available tools:` セクションにも追記する:

```
- search_code(pattern, [file_pattern]): Search for a pattern in all files
```

### 3.3 テスト
既存 `chat_test.go` に追加する。

- `buildSystemPrompt(true)` に `search_code` が含まれること
- `buildToolDefinitions` の結果に `search_code` が含まれること
- 既存 Tool 群の順序や存在が壊れていないこと
---
## Step 4: 安全性と制限
### 4.1 セキュリティ
Phase 1/2 と同じく `artifacts.Manager` の検証を経由する:

- `search_code` は workspace 内のファイルのみを対象にする
- `SearchFiles` 内で `filepath.EvalSymlinks` による symlink 解決を実施し、ワークスペース外のファイルは走査しない
- `file_pattern` 引数は `filepath.Match` で処理され、パス区切りを含まない単純 glob のみ受け付ける
- 検索は read-only であり、workspace に追加ファイルを生成しない

### 4.2 実行制約
- `search_code` の結果数上限: 50 件（`maxSearchResults`）
- `search_code` のファイルサイズ上限: 1MB（`maxSearchFileSize`）
- `ExecuteWithContext` の 30 秒タイムアウトが適用される
- バイナリファイルは検索対象外

### 4.3 モード共存
- Phase 1/2 と同様、Markdown モードと Agent モードは並列に共存する
- `search_code` は Agent モードでのみ利用可能
- セッション履歴には他の Tool Call と同様に保存される
---
## Step 5: テストと確認
### 推奨コマンド
```powershell
mise exec -- go test -v ./artifacts/...
mise exec -- go test -v ./tools/...
mise exec -- go test -v ./...
mise run test:verbose
mise run build
```

### 期待する検証範囲
- `artifacts`: SearchFiles の正規表現検索、フィルタリング、バイナリスキップ、結果上限、結果順安定化
- `tools/search_code_tool`: 引数検証、結果フォーマット
- `chat`: System Prompt に `search_code` が含まれること、`buildToolDefinitions` に `search_code` が含まれること

### 手動確認
1. Agent モードで `search_code` が発行される（コード検索指示で確認）
2. `file_pattern` によるフィルタリングが動作する（例: `"*.js"` のみ検索）
3. Phase 1/2 の既存 Tool が従来通り動作する
4. Markdown モードが影響なく動作する
5. Phase 3 実装により workspace に内部管理用ファイルが増えない
---
## 将来の拡張候補
- 相対パス全体に対する glob フィルタ
- 行範囲付き `read_file`
- セマンティック検索
- project index / symbol index の再検討（必要性が確認できた場合のみ）
---
## 関連ドキュメント
- `docs/03_agent_update.md` — エージェント拡張設計書（フェーズ定義）
- `docs/04_agent_specification.md` — Phase 1 実装仕様書
- `docs/05_agent_specification_2.md` — Phase 2 実装仕様書
---
**バージョン**: 1.1.0
**作成日**: 2026-04-05
**最終更新**: 2026-04-05 (`generate_index` / project index file を Phase 3 から除外し、`search_code` 単体仕様へ再整理)
**対象フェーズ**: Phase 3 (Code Search)
**前提フェーズ**: Phase 1 (MVP) ✅ 完了 / Phase 2 (Diff-Based Edit) ✅ 主経路完了
