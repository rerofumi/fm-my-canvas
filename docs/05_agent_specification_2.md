# 05. エージェント機能 Phase 2 実装仕様書 (Diff-Based Edit Specification)

本ドキュメントは `docs/03_agent_update.md` v1.3.0 の Phase 2 を実装するための具体仕様書である。
Phase 1 (`docs/04_agent_specification.md`) が完了していることを前提とする。

---

## 用語定義

| 用語 | 意味 |
|------|------|
| **apply_edit** | Search/Replace 方式による部分編集 Tool。既存ファイル内の検索文字列を置換文字列に差し替える |
| **apply_diff** | Unified diff 方式による部分編集 Tool（オプション）。標準 unified diff 形式のパッチを適用する |
| **Edit Engine** | `apply_edit` の search/replace 適用ロジックを担う内部エンジン |
| **Diff Engine** | `apply_diff` の unified diff 解析・適用ロジックを担う内部エンジン（オプション） |
| **フォールバック** | `apply_edit` / `apply_diff` の適用失敗時、LLM にエラーをフィードバックし `write_file` での完全再生成を促す戦略 |
| **context window 管理** | Tool 結果の要約置換や古い Tool 結果の除外により、LLM への送信トークン数を管理する仕組み |

---

## Phase 2 の境界

### In Scope

- `apply_edit` Tool の実装
- `apply_diff` Tool の実装（オプション）
- Edit Engine (`tools/edit_engine.go`) の実装
- Diff Engine (`tools/diff_engine.go`) の実装（オプション）
- System Prompt の Phase 2 更新
- Tool 結果の context window 管理強化（古い Tool 結果の要約置換）
- 差分適用失敗時の `write_file` フォールバック戦略

### Out of Scope

- `search_code` Tool（Phase 3）
- コードインデックス、セマンティック検索（Phase 3）
- 並列 Tool 呼び出し最適化
- マルチエージェントアーキテクチャ（Phase 4）
- 行範囲付き `read_file`（Phase 3 以降で検討）

### 前提条件

- Phase 1 が完了し、以下が実装済みであること:
  - `read_file`, `write_file`, `list_files` Tool
  - `provider.Provider.StreamWithTools`（Ollama / OpenRouter）
  - `ChatService.sendMessageWithTools` による Tool Call ループ
  - `tool-event` による Tool 実行状態の可視化
  - `RestoreArtifacts` のファイルシステム基準化
  - Tool 結果の 50KB 切り詰め

### 受け入れ条件

- `apply_edit` による search/replace 編集が正しく適用される
- `apply_edit` の適用失敗時（検索文字列が見つからない、複数マッチ）にエラーが LLM にフィードバックされる
- LLM がフォールバックとして `write_file` で完全再生成を試みることを System Prompt で誘導できる
- `apply_diff` はオプションであり、無効化していても `apply_edit` + `write_file` で支障なく動作する
- 古い Tool 結果の要約置換により、長時間の Tool Call ループでも context window を圧迫しにくくなる
- Phase 1 の既存 Tool（`read_file`, `write_file`, `list_files`）が従来通り動作する

### 実装前確認事項

- Phase 1 の受け入れ条件である「Tool Call メッセージがセッション履歴に保存されること」が満たされているかを先に確認する
- 未対応であれば、Phase 2 の着手前または同一ブランチ内で補完する
- `ToolManager.Tools()` の返却順が map 依存で不安定な場合、Tool 定義順が毎回変わるため固定順にする
- `apply_diff` はデフォルト無効とし、`apply_edit` を Phase 2 の主経路とする

---

## 変更ファイル一覧

```text
新規:
  tools/edit_engine.go
  tools/edit_engine_test.go
  tools/edit_apply_tool.go
  tools/edit_apply_tool_test.go
  tools/diff_engine.go             (オプション)
  tools/diff_engine_test.go        (オプション)
  tools/diff_apply_tool.go         (オプション)
  tools/diff_apply_tool_test.go    (オプション)

変更:
  chat.go
    ... apply_edit / apply_diff の Tool 登録追加
    ... buildSystemPrompt の Phase 2 更新
    ... context window 管理強化 (古い Tool 結果の要約置換)

  tools/registry.go
    ... Tools() の返却順序を安定化（任意）
```

---

## 推奨実装順序

1. `tools/edit_engine.go` — Edit Engine の純粋ロジック
2. `tools/edit_apply_tool.go` — `apply_edit` Tool の実装
3. `tools/diff_engine.go` — Diff Engine（オプション）
4. `tools/diff_apply_tool.go` — `apply_diff` Tool（オプション）
5. `chat.go` — Tool 登録、System Prompt 更新、context window 管理
6. 結合確認、手動確認

各ステップは **実装 → パッケージ単体テスト → 次ステップ** の順で進める。

---

## Step 1: `tools/edit_engine.go`

### 変更内容

Search/Replace の適用ロジックを独立したエンジンとして実装する。

```go
package tools

type EditEngine struct{}

type EditResult struct {
    Content     string
    MatchCount  int
    Replacement string
}

var errNoMatch = fmt.Errorf("search text not found in file")
var errMultipleMatches = fmt.Errorf("search text matches multiple locations in file")
var errEmptySearch = fmt.Errorf("search text must not be empty")

func NewEditEngine() *EditEngine

func (e *EditEngine) Apply(content, search, replace string) (string, error)

func (e *EditEngine) FindMatchCount(content, search string) int
```

### 要件

- `Apply` は `content` 内から `search` を検索し、**厳密に 1 箇所のみ**マッチする場合に `replace` で置換して返す
- マッチが 0 件の場合は `errNoMatch` を返す
- マッチが 2 件以上の場合は `errMultipleMatches` を返す
- `search` が空文字の場合は `errEmptySearch` を返す
- 置換後のファイルサイズが 1MB (`maxFileSize`) を超える場合はエラーを返す
- `FindMatchCount` はテスト用途で公開する
- LF / CRLF の正規化は行わず、`search` はファイル内容に対して**バイト列として完全一致**で判定する
- `search == replace` は成功扱いとし、内容差分がなくてもエラーにはしない
- エラー判定は呼び出し側テストから `errors.Is` で識別できるよう、wrap を許容する

### テスト

- 正常系: 1 箇所マッチ → 置換成功
- 0 マッチ → `errNoMatch`
- 2 箇所以上マッチ → `errMultipleMatches`
- 空 search → `errEmptySearch`
- 置換後サイズ超過 → エラー
- マッチしない文字列 → `errNoMatch`
- 複数行の search/replace → 正常動作
- search と replace が同一 → エラーにはならずそのまま返す（冪等性）
- CRLF を含むファイルで、改行コードが一致する場合のみマッチする
- `FindMatchCount` が `Apply` と同じ判定基準になる

---

## Step 2: `tools/edit_apply_tool.go`

### 変更内容

```go
package tools

type ApplyEditTool struct {
    manager *artifacts.Manager
    engine  *EditEngine
}

func NewApplyEditTool(m *artifacts.Manager) *ApplyEditTool
func (t *ApplyEditTool) Name() string        // "apply_edit"
func (t *ApplyEditTool) Description() string
func (t *ApplyEditTool) Parameters() map[string]any
func (t *ApplyEditTool) Execute(sessionID string, args map[string]any) (string, error)
```

### Parameters スキーマ

```json
{
  "name": "apply_edit",
  "description": "Apply a search/replace edit to an existing file. Finds the exact search text and replaces it with the replace text. The search text must match exactly one location in the file.",
  "parameters": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "The relative path of the file to edit."
      },
      "search": {
        "type": "string",
        "description": "The exact text to find in the file."
      },
      "replace": {
        "type": "string",
        "description": "The text to replace the search text with."
      }
    },
    "required": ["path", "search", "replace"]
  }
}
```

### Execute 処理フロー

1. `path`, `search`, `replace` を args から取得（不足時はエラー）
2. 各引数の型が `string` であることを明示的に検証する
3. `manager.ReadFile(sessionID, path)` で現在の内容を取得
4. `engine.Apply(content, search, replace)` で置換を実行
5. 成功したら `manager.WriteFile(sessionID, path, updated)` で書き戻す
6. 結果メッセージを返す:
   - 成功: `"Successfully edited <path> (1 replacement)"`
   - 失敗: エラーメッセージ（`errNoMatch`, `errMultipleMatches` 等）

### 要件

- ファイルが存在しない場合はエラー（新規作成には `write_file` を使用させる）
- `artifacts.Manager` のパスバリデーションを経由するため、パストラバーサルは自動的に防止される
- Tool 実行エラーは `sendMessageWithTools` の既存フローにより LLM にフィードバックされる
- 成功メッセージには path と置換件数を含め、UI と LLM の両方で解釈しやすくする
- 引数型不正（例: `search: 123`）は JSON デコード成功後でも Tool 側でエラーにする

### テスト

- 正常系: 既存ファイルの一部置換 → 成功メッセージとファイル更新
- ファイル未存在 → エラー
- search がマッチしない → エラー
- search が複数マッチ → エラー
- 引数不足（path / search / replace 各々）→ エラー
- 引数型不正（非 string）→ エラー
- パストラバーサル → エラー

---

## Step 3: `tools/diff_engine.go`（オプション）

### 変更内容

Unified diff 形式のパッチを解析・適用するエンジンを実装する。

```go
package tools

type DiffEngine struct{}

type Hunk struct {
    OldStart int
    OldCount int
    NewStart int
    NewCount int
    Lines    []DiffLine
}

type DiffLine struct {
    Type    byte // '+', '-', ' '
    Content string
}

func NewDiffEngine() *DiffEngine
func (e *DiffEngine) Parse(diff string) ([]Hunk, error)
func (e *DiffEngine) Apply(content string, hunks []Hunk) (string, error)
```

### 要件

- `Parse` は unified diff 文字列を `[]Hunk` に変換する
- `Apply` は元の `content` に hunks を順次適用する
- コンテキスト行（` `）が一致しない場合はエラーとする
- `@@ ... @@` ヘッダの行番号が実際と一致しない場合は警告を出しつつ内容ベースでマッチを試みる
- 適用後のファイルサイズが 1MB を超える場合はエラーとする

### テスト

- 1 行追加
- 1 行削除
- 1 行変更
- 複数 hunk の順次適用
- コンテキスト不一致 → エラー
- 不正な diff 形式 → パースエラー
- 空の diff → エラー

---

## Step 4: `tools/diff_apply_tool.go`（オプション）

### 変更内容

```go
package tools

type ApplyDiffTool struct {
    manager *artifacts.Manager
    engine  *DiffEngine
}

func NewApplyDiffTool(m *artifacts.Manager) *ApplyDiffTool
func (t *ApplyDiffTool) Name() string        // "apply_diff"
func (t *ApplyDiffTool) Description() string
func (t *ApplyDiffTool) Parameters() map[string]any
func (t *ApplyDiffTool) Execute(sessionID string, args map[string]any) (string, error)
```

### Parameters スキーマ

```json
{
  "name": "apply_diff",
  "description": "Apply a unified diff patch to an existing file. The diff should be in standard unified diff format.",
  "parameters": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "The relative path of the file to patch."
      },
      "diff": {
        "type": "string",
        "description": "The unified diff patch to apply."
      }
    },
    "required": ["path", "diff"]
  }
}
```

### 要件

- ファイルが存在しない場合はエラー
- diff のパース失敗・適用失敗はエラーメッセージとして返す
- 成功時: `"Successfully applied diff to <path> (<N> hunks applied)"`
- `apply_diff` は feature flag あるいは明示登録時のみ有効化し、Phase 2 のデフォルト経路にはしない

### テスト

- 正常系: 1 hunk の diff 適用 → 成功
- 複数 hunk → 成功
- 不正な diff 形式 → エラー
- ファイル未存在 → エラー
- 引数不足 → エラー

---

## Step 5: `chat.go`

### 5.1 Tool 登録の追加

`NewChatService` に `apply_edit`（およびオプションの `apply_diff`）を登録する。

```go
func NewChatService(artifactMgr *artifacts.Manager, server *artifacts.Server) (*ChatService, error) {
    // ... 既存処理 ...

    tm := tools.NewToolManager()
    tm.Register(tools.NewReadFileTool(artifactMgr))
    tm.Register(tools.NewWriteFileTool(artifactMgr))
    tm.Register(tools.NewListFilesTool(artifactMgr))
    tm.Register(tools.NewApplyEditTool(artifactMgr))
    // オプション: tm.Register(tools.NewApplyDiffTool(artifactMgr))

    // ...
}
```

### 5.2 System Prompt の Phase 2 更新

Agent モードの System Prompt に `apply_edit` の説明を追加する。

```go
func buildSystemPrompt(agentMode bool) string {
    if agentMode {
        return "You are a helpful coding assistant with file system access. You can read, write, and list files in the user's artifact workspace.\n\n" +
            "When asked to modify code:\n" +
            "1. First, use read_file to understand the current code\n" +
            "2. Analyze what needs to be changed\n" +
            "3. For minimal changes to existing code, use apply_edit to apply a search/replace edit\n" +
            "4. For large changes or new files, use write_file to write the full content\n" +
            "5. Always verify your changes make sense in the context of the whole project\n\n" +
            "When apply_edit fails (e.g., search text not found or multiple matches), the error will be reported back to you. " +
            "In that case, use write_file to rewrite the entire file as a fallback.\n\n" +
            "When asked about the project structure:\n" +
            "1. Use list_files to understand the file layout\n" +
            "2. Read relevant files to understand dependencies\n\n" +
            "Available tools:\n" +
            "- read_file(path): Read file contents\n" +
            "- write_file(path, content): Write file contents\n" +
            "- list_files([path]): List files in directory\n" +
            "- apply_edit(path, search, replace): Apply a search/replace edit to a file\n"
            // オプション: + "- apply_diff(path, diff): Apply a unified diff patch to a file\n"
    }
    // ... Markdown モードは変更なし ...
}
```

### 5.3 Context Window 管理の強化

長時間の Tool Call ループで古い Tool 結果が蓄積し context window を圧迫する問題に対処する。

#### 方針

直近 `N` ラウンド（`keepRecentRounds = 2`）の Tool 結果はそのまま保持し、それより古い `tool` ロールメッセージの `Content` を要約に置換して LLM に送信する。

ここでいう「ラウンド」は `assistant(tool_calls 付き)` 1 件と、それに続く `tool` ロール複数件のまとまりを指す。`tool` メッセージ件数では数えない。

```go
const keepRecentRounds = 2
const summaryPrefix = "[Previous tool result summarized] "

func summarizeOldToolResults(messages []types.Message) []types.Message {
    summarized := make([]types.Message, len(messages))
    copy(summarized, messages)

    toolIndices := []int{}
    for i, m := range summarized {
        if m.Role == types.RoleTool {
            toolIndices = append(toolIndices, i)
        }
    }

    if len(toolIndices) <= keepRecentRounds {
        return summarized
    }

    cutoff := len(toolIndices) - keepRecentRounds
    for _, idx := range toolIndices[:cutoff] {
        content := summarized[idx].Content
        firstLine := content
        if idx := strings.Index(content, "\n"); idx >= 0 {
            firstLine = content[:idx]
        }
        if len(firstLine) > 100 {
            firstLine = firstLine[:100] + "..."
        }
        summarized[idx] = types.Message{
            Role:       summarized[idx].Role,
            Content:    summaryPrefix + firstLine,
            ToolCallID: summarized[idx].ToolCallID,
            CreatedAt:  summarized[idx].CreatedAt,
        }
    }

    return summarized
}
```

#### sendMessageWithTools への適用

`StreamWithTools` を呼ぶ前に `summarizeOldToolResults` を適用する。

```go
// Tool Call ループ内の StreamWithTools 呼び出し箇所:
messagesForLLM := summarizeOldToolResults(allMessages)
err := p.StreamWithTools(ctx, messagesForLLM, toolDefs, func(event provider.StreamEvent) {
    // ...
})
```

**注意**: `summarizeOldToolResults` は LLM 送信用の一時コピーを生成し、`allMessages`（セッション保存用）は改変しない。

**実装メモ**:

- Phase 1 実装に合わせ、`sendMessageWithTools` 内で `allMessages` をそのまま `StreamWithTools` に渡している箇所を、要約済みコピーに差し替える
- 要約対象の本文は 1 行目だけでなく、`tool_name` や success / failure が判別できる粒度を残す
- `read_file` の結果を要約する場合でも、「どのファイルを読んだ結果か」が失われないよう path を残す

### 5.4 テスト

既存 `chat_test.go` に追加する。

- `buildSystemPrompt(true)` に `apply_edit` が含まれること
- `summarizeOldToolResults` で古い tool メッセージが要約されること
- `summarizeOldToolResults` で最近のラウンドの tool メッセージが保持されること
- `summarizeOldToolResults` が元のスライスを改変しないこと
- tool メッセージが 0 件の場合に panic しないこと
- 1 ラウンド内に複数 Tool Call がある場合でも、ラウンド単位で保持 / 要約が判定されること
- 要約後のメッセージで `ToolCallID` と `CreatedAt` が維持されること

---

## Step 6: 安全性と制限

### 6.1 セキュリティ

Phase 1 と同じく `artifacts.Manager.validateWorkspacePath` を経由するため、以下は自動的に適用される:

- 相対パスのみ許可
- `../` による workspace 外アクセス禁止
- 1MB ファイルサイズ上限

追加の制約:

- `apply_edit` は既存ファイルのみ対象（新規作成は `write_file` を使用）
- `apply_diff` も既存ファイルのみ対象
- 置換後・適用後のファイルサイズが 1MB を超える場合は書き込みを拒否する

### 6.2 フォールバック戦略

差分適用の失敗は珍しくないため、LLM が自己修復できるよう System Prompt で誘導する:

1. `apply_edit` 失敗 → エラーが `tool` ロールメッセージとして LLM に返る
2. LLM はエラー内容（マッチ数等）を確認し、`read_file` で最新のファイル内容を取得し直す
3. LLM が再度 `apply_edit` を試みるか、`write_file` で完全再生成にフォールバックする
4. このループは `maxToolRounds`（10 回）の範囲内で行われる

**補足**:

- `apply_edit` が 2 回以上連続で失敗した場合は `write_file` を優先する旨を prompt に含めてよい
- `apply_diff` 失敗時も同様に、`read_file` のやり直しまたは `write_file` への移行を促す

### 6.3 モード共存

- Phase 1 と同様、Markdown モードと Agent モードは並列に共存する
- `apply_edit` / `apply_diff` は Agent モードでのみ利用可能
- セッション履歴には Tool Call メッセージとして保存される

---

## Step 7: テストと確認

### 推奨コマンド

```powershell
mise exec -- go test -v ./tools/...
mise exec -- go test -v ./
mise run test:verbose
mise run build
```

### 期待する検証範囲

- `tools/edit_engine`: search/replace の正確性、エッジケース（0 マッチ、複数マッチ、空 search）
- `tools/edit_apply_tool`: Tool としての引数検証、ファイル I/O 経由の成功/失敗
- `tools/diff_engine`（オプション）: unified diff のパースと適用
- `tools/diff_apply_tool`（オプション）: Tool としての引数検証、ファイル I/O 経由の成功/失敗
- `chat`: System Prompt に `apply_edit` が含まれる、古い Tool 結果の要約置換、複数 Tool Call を含む 1 ラウンドの扱い
- `tools/registry`: `Tools()` の返却順が安定していること

### 手動確認

1. Agent モードで `apply_edit` が発行される（小さな変更指示で確認）
2. `apply_edit` 失敗時に LLM が `write_file` にフォールバックする
3. Phase 1 の `read_file` / `write_file` / `list_files` が従来通り動作する
4. Markdown モードが影響なく動作する
5. 長時間ループで古い Tool 結果が要約される（デバッグログ等で確認）

---

## 関連ドキュメント

- `docs/03_agent_update.md` — エージェント拡張設計書（フェーズ定義）
- `docs/04_agent_specification.md` — Phase 1 実装仕様書

---

**バージョン**: 1.0.0  
**作成日**: 2026-04-05  
**対象フェーズ**: Phase 2 (Diff-Based Edit)  
**前提フェーズ**: Phase 1 (MVP) 完了
