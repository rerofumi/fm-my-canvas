# 04. エージェント機能 実装仕様書 (Phase 1 Implementation Specification)

本ドキュメントは `docs/03_agent_update.md` v2.0.0 の Phase 1 (MVP) を実装するための具体仕様書である。
`03` に追記された進行順序、`tool-event` 分離、`RestoreArtifacts` の扱い、安全性制約、キャンセル/タイムアウト、context window 管理を本書に反映する。

**ステータス**: ✅ Phase 1 実装完了（2026-04-05 確認済み）

---

## 用語定義

| 用語 | 意味 |
|------|------|
| **Markdown モード** | 既存の動作。LLM のテキストレスポンスからコードブロックを抽出して Artifact を復元する方式 |
| **Agent モード** | 新規追加。LLM が Tool Call を発行し、バックエンドがワークスペース内ファイルを直接読み書きする方式 |
| **Tool Call ラウンド** | LLM へのリクエスト → Tool Call 受信 → Tool 実行 → Tool 結果を LLM に返す、の 1 サイクル |
| **Tool 結果の切り詰め** | LLM に再送する Tool 実行結果を最大 50KB に抑え、中間を省略する Phase 1 の context window 対策 |

---

## Phase 1 の境界

### In Scope

- `read_file`, `write_file`, `list_files`
- `provider.Provider.StreamWithTools` の追加と Ollama / OpenRouter 実装
- `ChatService.SendMessage` のモード分岐
- `ChatService.sendMessageWithTools` による Tool Call ループ
- `tool-event` による Tool 実行状態の可視化
- `RestoreArtifacts` のファイルシステム基準化
- `artifacts.Manager` での共通パス検証
- Tool 結果の 50KB 切り詰め

### Out of Scope

- `apply_edit`, `apply_diff`, `search_code`
- Edit / Diff Engine
- 並列 Tool 呼び出し
- マルチエージェント
- 古い Tool 結果の要約置換や行範囲付き `read_file`

### 受け入れ条件 — ✅ 全項目確認済み

- ✅ Markdown モードが従来通り壊れず動作する
- ✅ Agent モードでは Tool Call によりファイル編集が完了する
- ✅ Tool Call メッセージがセッション履歴に保存される
- ✅ `RestoreArtifacts` は Markdown 解析ではなくファイルシステムから復元する
- ✅ `llm-event` の既存 payload 形式は維持される

---

## 変更ファイル一覧

```text
変更:
  types/types.go
    ... RoleTool, ToolCall, Message.tool_calls, Message.tool_call_id
    ... ArtifactFileInfo 型追加 (Path, Language, Content)

  artifacts/manager.go
    ... ReadFile 追加
    ... validateWorkspacePath 追加
    ... WriteFile / ListFiles に共通パス検証とサイズ制限を適用
    ... NewManagerWithDir 追加（テスト用）
    ... ListFiles で filepath.EvalSymlinks による symlink 解決追加

  config/config.go
    ... AgentMode フィールド追加

  provider/provider.go
    ... StreamEvent, ToolDefinition, StreamWithTools 追加

  provider/ollama.go
    ... StreamWithTools 実装
    ... tool_calls の arguments を JSON string に正規化
    ... 空 Tool Call ID に ollama_tc_<index> 仮 ID を付与

  provider/openrouter.go
    ... StreamWithTools 実装
    ... delta.tool_calls の蓄積
    ... baseURL 差し替え対応（テスト用）

  chat.go
    ... SendMessage のモード分岐
    ... sendMessageWithTools 追加
    ... sendMessageMarkdown に cancel context 追加
    ... buildSystemPrompt 系の切替 (Phase 2 用に apply_edit を含む)
    ... buildToolDefinitions 追加
    ... RestoreArtifacts を resolveArtifactInfo ヘルパー経由に変更
    ... CancelSend 追加 (cancelMu + cancelFn によるスレッドセーフ)
    ... GetArtifactFileContents 追加 (Code タブのディスク表示用)
    ... languageFromExt 追加 (拡張子 → 言語 ID マッピング)
    ... newProvider 追加 (Provider 生成の重複排除)
    ... tool-event emit, timeout, truncation 追加
    ... summarizeOldToolResults 追加 (Phase 2 で実装、Phase 1 から利用)

  artifacts/server.go
    ... consoleInterceptorJS 追加 (iframe console 傍受スクリプト)
    ... injectConsoleInterceptor 追加 (<head> / <HEAD> / <html> 直後に注入)
    ... generateDirectoryListing 追加 (index.html 不在時のファイル一覧 HTML)
    ... cachedFileServer 追加 (no-cache ヘッダ + HTML 注入 + ディレクトリリスト)

  session/manager.go
    ... strings.TrimSuffix 使用
    ... 自動タイトル設定 ("New Chat" → 最初のユーザーメッセージ先頭50文字)

  frontend/src/lib/stores/chat.svelte.ts
    ... toolCallLog state 追加
    ... ConsoleLogEntry / consoleLogs state 追加
    ... 各 getter / setter 関数 (Svelte 5 runes 対応)

  frontend/src/lib/services/wails.ts
    ... tool-event リスナー追加
    ... llm-event の後方互換維持
    ... loadArtifactFilesFromDisk 追加 (ディスク上のファイル内容を Code タブ用に取得)
    ... cancelSend 追加 (CancelSend Go メソッド呼び出し)
    ... initGlobalConsoleCapture 追加 (console.* 傍受 + iframe postMessage リスナー)
    ... scheduleArtifactUpdate 追加 (400ms throttle でストリーミング中の artifact 更新)

  frontend/src/components/chat/ChatArea.svelte
    ... ToolCallMessage の表示
    ... tool / system ロールの生メッセージを通常表示から除外
    ... cancelSend / handleStop 追加

  frontend/src/components/chat/ChatInput.svelte
    ... onstop prop 追加
    ... パイロットランプ (緑色パルスアニメ) + Stop ボタン追加
    ... 自動リサイズ (max 150px)

  frontend/src/components/chat/ChatMessage.svelte
    ... code block path= パース対応 (既存)

  frontend/src/components/layout/SettingsModal.svelte
    ... Agent モード切替 UI 追加

新規:
  tools/tool.go
  tools/registry.go
  tools/file_read_tool.go
  tools/file_write_tool.go
  tools/file_list_tool.go
  tools/edit_engine.go               # Phase 2 で追加
  tools/edit_apply_tool.go           # Phase 2 で追加
  frontend/src/components/chat/ToolCallMessage.svelte
  frontend/src/components/artifacts/ConsolePane.svelte  # Console ログ表示

テスト:
  types/types_test.go
  artifacts/manager_test.go
  tools/registry_test.go
  tools/file_read_tool_test.go
  tools/file_write_tool_test.go
  tools/file_list_tool_test.go
  tools/edit_engine_test.go           # Phase 2 で追加
  tools/edit_apply_tool_test.go       # Phase 2 で追加
  provider/ollama_test.go
  provider/openrouter_test.go
  chat_test.go
```

---

## 推奨実装順序

1. `types/types.go`
2. `artifacts/manager.go`
3. `tools/`
4. `provider/provider.go` と `provider/ollama.go`
5. `provider/openrouter.go`
6. `chat.go`
7. `frontend/`
8. 結合確認、手動確認

各ステップは **実装 → パッケージ単体テスト → 次ステップ** の順で進める。

---

## Step 1: `types/types.go`

### 変更内容

```go
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleSystem    Role = "system"
    RoleTool      Role = "tool"
)

type ToolCall struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}

type Message struct {
    Role       Role       `json:"role"`
    Content    string     `json:"content"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
    CreatedAt  string     `json:"created_at"`
}
```

### 要件

- `tool_calls`, `tool_call_id` は `omitempty` を付け、既存セッション JSON と後方互換を保つ
- `ToolCall.Arguments` は Provider 差異を吸収するため JSON string に統一する

### テスト

- `RoleTool == "tool"`
- `ToolCalls` を含む `Message` の marshal / unmarshal
- `ToolCallID` を含む `tool` ロールの marshal / unmarshal
- 旧フォーマットのセッション JSON が問題なく読めること

---

## Step 2: `artifacts/manager.go`

### 変更内容

Phase 1 では `ReadFile` の追加だけでは不十分であり、**Read / Write / List のすべてに同一のパス制約を適用する**。

```go
func (m *Manager) validateWorkspacePath(sessionID, filename string) (string, error)
func (m *Manager) ReadFile(sessionID, filename string) (string, error)
func (m *Manager) WriteFile(sessionID, filename, content string) error
func (m *Manager) ListFiles(sessionID string) ([]string, error)
func NewManagerWithDir(baseDir string) *Manager
```

### 要件

- `validateWorkspacePath` は公開しない内部ヘルパーとする
- 相対パスのみ許可し、`../` による workspace 外アクセスを拒否する
- `ReadFile` は存在確認、ディレクトリ拒否、1MB 上限を持つ
- `WriteFile` は同じ検証を通した上で 1MB 上限を持ち、atomic write を維持する
- `ListFiles` は返却する各パスが workspace 内相対パスであることを保証する
- workspace 外を指すシンボリックリンク経由の参照は拒否する
- `NewManagerWithDir` はテスト容易性のために追加する

### テスト

- `ReadFile` 成功 / 未存在 / ディレクトリ / パストラバーサル / サイズ超過
- `WriteFile` 成功 / パストラバーサル / 内容サイズ超過
- `ListFiles` が workspace 外を返さないこと
- `NewManagerWithDir` を使って一時ディレクトリで検証できること

---

## Step 3: `config/config.go`

### 変更内容

```go
type Config struct {
    Provider         string `json:"provider"`
    OllamaEndpoint   string `json:"ollama_endpoint"`
    OllamaModel      string `json:"ollama_model"`
    OpenRouterAPIKey string `json:"openrouter_api_key"`
    OpenRouterModel  string `json:"openrouter_model"`
    AgentMode        bool   `json:"agent_mode"`
}
```

### 要件

- `AgentMode` の既定値は `false`
- 既存 `config.json` に `agent_mode` がなくても互換性を保つ

---

## Step 4: `tools/`

### 4.1 `tools/tool.go`

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Execute(sessionID string, args map[string]any) (string, error)
}
```

### 4.2 `tools/registry.go`

```go
type ToolManager struct {
    registry map[string]Tool
}

func NewToolManager() *ToolManager
func (m *ToolManager) Register(tool Tool)
func (m *ToolManager) Tools() []Tool
func (m *ToolManager) Execute(sessionID string, tc types.ToolCall) (string, error)
func (m *ToolManager) ExecuteWithContext(ctx context.Context, sessionID string, tc types.ToolCall) (string, error)
```

### 4.3 各 Tool

- `ReadFileTool`: `artifacts.Manager.ReadFile` を呼ぶ
- `WriteFileTool`: `artifacts.Manager.WriteFile` を呼ぶ
- `ListFilesTool`: `path` 引数がある場合は表示対象を絞り込み、結果を整形して返す

### 要件

- `ToolCall.Arguments` の JSON デコードは `ToolManager` 側で行う
- 不明な Tool 名、JSON 不正、引数不足は明示的エラーにする
- `ExecuteWithContext` は Tool インターフェースを無理に変更せず、タイムアウト制御用ラッパーとして実装してよい

### テスト

- Tool 登録 / 実行ディスパッチ
- 不明 Tool / 不正 JSON
- `read_file`, `write_file`, `list_files` の成功 / 失敗

---

## Step 5: `provider/provider.go`

### 変更内容

```go
type StreamEventType string

const (
    EventContent  StreamEventType = "content"
    EventToolCall StreamEventType = "tool_call"
    EventDone     StreamEventType = "done"
)

type StreamEvent struct {
    Type      StreamEventType
    Content   string
    ToolCalls []types.ToolCall
}

type ToolDefinition struct {
    Type     string `json:"type"`
    Function struct {
        Name        string `json:"name"`
        Description string `json:"description"`
        Parameters  any    `json:"parameters"`
    } `json:"function"`
}

type Provider interface {
    Stream(ctx context.Context, messages []types.Message, cb func(chunk string)) error
    StreamWithTools(ctx context.Context, messages []types.Message, tools []ToolDefinition, cb func(event StreamEvent)) error
}
```

### 要件

- Provider は **ストリーム解析のみ** を担当し、Tool Call の実行ループは `ChatService` 側で管理する
- `Stream` は Markdown モード用として維持する

---

## Step 6: `provider/ollama.go`

### 要件

- `message.tool_calls[].function.arguments` は `map[string]any` で受け取る
- Tool Call 受信時に `json.Marshal` して `ToolCall.Arguments` に格納する
- Tool Call ID が空なら、少なくとも同一レスポンス内で一意な仮 ID を付与する
- `done == true` のとき `EventDone` を emit する

### テスト

- テキストのみストリーム
- Tool Call を含むストリーム
- テキストと Tool Call の混在
- 空の Tool Call ID でも処理できること

---

## Step 7: `provider/openrouter.go`

### 要件

- `choices[0].delta.tool_calls` を `index` ごとに蓄積する
- `finish_reason == "tool_calls"` で `EventToolCall` を emit する
- `data: [DONE]` で `EventDone` を emit する
- テストのために `OpenRouterProvider` に `baseURL` 差し替え機構を持たせる

### テスト

- テキストのみストリーム
- `delta.tool_calls` の分割蓄積
- 複数 Tool Call (`index: 0`, `index: 1`) の正しい結合
- `Arguments` が最終的に妥当な JSON string になること

---

## Step 8: `chat.go`

### 8.1 モード分岐

`SendMessage` は既存フローを壊さず、`AgentMode` に応じて Markdown / Agent を切り替える。

```go
func (c *ChatService) SendMessage(sessionID string, message string) error {
    // ユーザメッセージ保存
    // セッション取得

    if c.config.AgentMode {
        return c.sendMessageWithTools(sessionID, allMessages)
    }
    return c.sendMessageMarkdown(sessionID, allMessages)
}
```

### 8.2 System Prompt 切替

- `buildSystemPrompt(agentMode bool)` を用意する
- Agent モードでは **実装済みの Tool のみ** を列挙する（Phase 2 時点: `read_file`, `write_file`, `list_files`, `apply_edit`）
- Markdown モードの既存 prompt は維持する

### 8.3 `sendMessageWithTools`

Phase 1 の中核は Tool Call ループである。以下の責務を 1 か所に集約する。

```go
const maxToolRounds = 10
const toolLoopTimeout = 5 * time.Minute
const maxToolResultBytes = 50 * 1024
```

#### 主要責務

1. `context.WithTimeout` でループ全体を 5 分に制限する
2. `newProvider()` で Provider を選択し、`StreamWithTools` を呼ぶ
3. `llm-event` では既存通り `chunk`, `done`, `error` のみを emit する
4. Tool 呼び出し開始 / 終了は `tool-event` で emit する
5. Tool Call があれば assistant メッセージ (`tool_calls` 付き) を履歴に追加する
6. Tool 実行結果を `tool` ロールのメッセージとして履歴に追加する
7. Tool 結果は LLM に再投入する前に最大 50KB へ切り詰める
8. `StreamWithTools` 呼び出し前に `summarizeOldToolResults` で古い tool 結果を要約置換する
9. Tool Call がなければ最終 assistant メッセージとして確定する
10. ループ終了後に `artifact-update` を `emitArtifactUpdate` 経由で emit する
11. `maxToolRounds` 到達時は最後の assistant メッセージ内容を `textAccumulatedOrDefault` で抽出し、エラーを返す

#### イベントプロトコル

| イベント | payload | 備考 |
|---------|---------|------|
| `llm-event` | `{type, content, session_id} map[string]string` | `chunk`, `done`, `error` のみ。既存互換を維持 |
| `tool-event` | `{type:"tool_call", tool_name, tool_args, session_id} map[string]any` | Tool 開始通知 |
| `tool-event` | `{type:"tool_result", tool_name, result, success, session_id} map[string]any` | `success` は追加フィールドとして扱ってよい |
| `artifact-update` | `{session_id, preview_url, files} map[string]string` | 既存形式を維持 |

#### エラーとフォールバック

- Tool 実行エラーは `tool` ロールメッセージとして LLM に返し、次ラウンドで自己修復させる
- `maxToolRounds` 到達時はエラーメッセージ ("reached maximum tool call rounds") を emit して打ち切る
- Tool Call 非対応モデルによる `400` などの Provider エラーは `llm-event(error)` として通知する
- Tool Call が 0 件で通常テキストだけ返るケースは、そのまま通常完了として扱う

### 8.4 `RestoreArtifacts`

`RestoreArtifacts` は Markdown 解析ベースではなく、**常にファイルシステムをソースオブトゥルースとする**。内部で `resolveArtifactInfo` ヘルパーを利用する。

```go
func (c *ChatService) resolveArtifactInfo(sessionID string) (files []string, previewURL string, ok bool) {
    files, err := c.artifact.ListFiles(sessionID)
    if err != nil || len(files) == 0 {
        return nil, "", false
    }

    wsDir := c.artifact.WorkspaceDir(sessionID)
    url, serr := c.server.Start(c.ctx, wsDir)
    if serr != nil {
        return nil, "", false
    }
    c.server.UpdateDir(wsDir)

    previewURL = url
    for _, f := range files {
        if f == "index.html" {
            previewURL = url + "/index.html"
            break
        }
    }
    return files, previewURL, true
}

func (c *ChatService) RestoreArtifacts(sessionID string) map[string]string {
    files, previewURL, ok := c.resolveArtifactInfo(sessionID)
    if !ok {
        return map[string]string{}
    }

    result := map[string]string{"files": strings.Join(files, ",")}
    if previewURL != "" {
        result["preview_url"] = previewURL
    }
    return result
}
```

**補足**: `index.html` が存在しない場合、`previewURL` はサーバーのルート URL となり、`generateDirectoryListing` によりファイル一覧ページが表示される。

### 8.5 キャンセル機能

ユーザーキャンセルをサポートするため、`ChatService` に以下を追加する。

```go
type ChatService struct {
    // ... 既存フィールド ...
    cancelMu sync.Mutex
    cancelFn context.CancelFunc
}

func (c *ChatService) CancelSend()
func (c *ChatService) setCancelFn(fn context.CancelFunc)
func (c *ChatService) clearCancelFn()
```

- `sendMessageMarkdown` と `sendMessageWithTools` の両方で `setCancelFn` / `clearCancelFn` を使用
- `cancelMu` によりスレッドセーフに管理
- フロントエンドの `ChatInput.svelte` Stop ボタンから `cancelSend()` を呼び出し

### 8.6 追加の Wails バインディング

```go
func (c *ChatService) GetArtifactFileContents(sessionID string) []types.ArtifactFileInfo
```

- セッション内全ファイルの Path / Language / Content を返す
- フロントエンドの Code タブがディスク上のファイル内容を表示するために使用
- `languageFromExt(path)` ヘルパーで拡張子から言語 ID を決定

### 8.5 テスト

`chat_test.go` を追加し、モック Provider / モック ToolManager で以下を検証する。

- Tool Call なしで 1 ラウンド完了
- `read_file` → `write_file` → 最終レスポンスの複数ラウンド
- Tool 実行エラーのフィードバック
- `maxToolRounds` 到達で打ち切り
- `tool-event` と `artifact-update` の emit
- Tool 結果が 50KB で切り詰められること

---

## Step 9: フロントエンド

### 9.1 `frontend/src/lib/stores/chat.svelte.ts`

- `toolCallLog` state を追加する (`ToolCallLogEntry[]`)
- `ConsoleLogEntry` / `consoleLogs` state を追加する
- `clearArtifactData()` で `toolCallLog` も初期化する
- Svelte 5 runes (`$state`, `$derived`) を使用
- getter / setter 関数パターンで外部アクセスを提供

```typescript
interface ToolCallLogEntry {
    toolName: string;
    toolArgs: string;
    status: 'running' | 'success' | 'error';
    result?: string;
    timestamp: number;
}

interface ConsoleLogEntry {
    type: 'log' | 'error' | 'warn' | 'info';
    message: string;
    timestamp: string;
    source: 'app' | 'iframe';
}
```

### 9.2 `frontend/src/lib/services/wails.ts`

- `llm-event` リスナーは既存の `chunk` / `done` / `error` のみ処理する
- `tool-event` リスナーを追加し、`tool_call` / `tool_result` を処理する
- `artifact-update` で `loadArtifactFilesFromDisk(sessionId)` を呼び出し、ディスク上のファイル内容を取得
- `cancelSend()` で `CancelSend()` Go メソッドを呼び出す
- `initGlobalConsoleCapture()` でアプリ自身の console を傍受し、iframe の `postMessage` をリッスン
- `scheduleArtifactUpdate()` で 400ms throttle によりストリーミング中の artifact 更新を実行

### 9.3 `frontend/src/components/chat/ToolCallMessage.svelte`

新規コンポーネントとして以下を提供する。

- Tool 呼び出しログの一覧表示 (`toolCallLog` ストアから取得)
- 実行中 (spinner) / 成功 (checkmark) / 失敗 (X) のステータス表示
- 引数と結果の折りたたみ表示 (`<details>` 要素)
- 複数ラウンドの進行状況表示

### 9.4 `frontend/src/components/chat/ChatArea.svelte`

- `tool` ロールと `system` ロールの生メッセージは通常のチャット本文としては表示しない
- `toolCallLog.length > 0` の場合に `ToolCallMessage` を表示
- `cancelSend` / `handleStop` を ChatInput に渡す
- ストリーミング中の blinking cursor 表示

### 9.5 `frontend/src/components/chat/ChatInput.svelte`

- `onsend`, `onstop`, `disabled` props を受け取る
- `disabled` 時 (ストリーミング中): パイロットランプ (緑色パルスアニメ) + "Working..." + Stop ボタンを表示
- 待機中: Send ボタンを表示
- Ctrl+Enter で送信
- textarea 自動リサイズ (max 150px)

### 9.6 `frontend/src/components/artifacts/ConsolePane.svelte`

新規コンポーネントとして以下を提供する。

- Console ログのリアルタイム表示 (`consoleLogs` ストアから取得)
- フィルタ (All / Log / Error / Warn / Info)
- Clear ボタン
- タイムスタンプ + タイプ表示
- iframe 由来のログには `[preview]` タグを表示
- 自動スクロール

### 9.7 `frontend/src/components/layout/SettingsModal.svelte`

- Provider 設定の近くに Agent モード切替を追加する
- ヒント文: "Uses Tool Calls for file operations instead of Markdown code blocks."

---

## Step 10: 安全性と制限

### 10.1 セキュリティ

1. **共通パスバリデーション**
   - 相対パスのみ許可
   - `../` による workspace 外アクセスを禁止
   - `read_file`, `write_file`, `list_files` すべてに共通適用

2. **サイズ制限**
   - 読み取り: 最大 1MB
   - 書き込み内容: 最大 1MB
   - Tool 結果の LLM 再投入: 最大 50KB

3. **Write 系制約**
   - 検証済みパスのみ書き込み可能
   - atomic write を維持

4. **ファイルタイプ制限**
   - Phase 1 ではデフォルト無効
   - 将来オプションで allowlist を有効化可能な設計にしてよい

### 10.2 実行制約

- Tool Call ラウンド数は 1 メッセージあたり最大 10
- Tool 実行は sequential のみ
- 全体タイムアウトは 5 分
- 個別 Tool タイムアウトは 30 秒
- ユーザーキャンセル時は `context.Cancel()` により中断する

### 10.3 モード共存

- Markdown モードと Agent モードは並列に共存する
- セッション復元はモードに依存せずファイルシステムから行う
- Agent モードのセッション履歴には Tool Call メッセージを保存する

---

## Step 11: テストと確認 — ✅ 全パッケージテスト通過済み

### 推奨コマンド

```powershell
mise exec -- go test -v ./types/...
mise exec -- go test -v ./artifacts/...
mise exec -- go test -v ./tools/...
mise exec -- go test -v ./provider/...
mise exec -- go test -v ./
mise run test:verbose
mise run build
```

### 期待する検証範囲 — ✅ 全項目検証済み

- ✅ `types`: JSON 往復と後方互換
- ✅ `artifacts`: パストラバーサル拒否、サイズ制限、Read/Write/List、symlink 解決
- ✅ `tools`: Dispatch、引数検証、各 Tool の成功 / 失敗
- ✅ `provider`: Ollama / OpenRouter の StreamWithTools
- ✅ `chat`: Tool Call ループ、event emit、RestoreArtifacts、truncation、cancel

### 手動確認 — ✅ 全項目確認済み

1. ✅ AgentMode OFF で Markdown モードが従来通り動く
2. ✅ AgentMode ON で Tool Call が発行される
3. ✅ Tool 実行ログが `ToolCallMessage` に表示される
4. ✅ セッション再読込時に `RestoreArtifacts` で preview が復元される
5. ✅ Stop ボタンで Tool Call ループをキャンセルできる
6. ✅ Console タブにアプリ / iframe の console 出力が表示される
7. ✅ Code タブにディスク上のファイル一覧・内容が表示される

---

**バージョン**: 2.0.0  
**作成日**: 2026-04-04  
**最終更新**: 2026-04-05 (Phase 1 + Phase 2 主経路 実装完了を反映: 追加コンポーネント、CancelSend、console capture、GetArtifactFileContents、resolveArtifactInfo 等を追記)
**対象フェーズ**: Phase 1 (MVP) — ✅ 完了
**対象元ドキュメント**: `docs/03_agent_update.md` v2.0.0
