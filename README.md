# fm-my-canvas

[English](README.en.md)

LLM のチャット対話を通じて HTML/CSS/JS のプロトタイプをリアルタイムに生成・プレビューできるデスクトップアプリケーション。

「ChatGPT の Artifact や Claude の Artifacts のような体験を、ローカル LLM でも手軽に使いたい」という動機から開発しました。Ollama や OpenRouter に接続し、会話しながら UI をプロトタイピングできます。

## 特徴

- **LLM チャットインターフェース** — Ollama (ローカル) または OpenRouter (クラウド) に対応したストリーミングチャット
- **リアルタイム Artifact プレビュー** — LLM が生成した HTML/CSS/JS を iframe サンドボックス内で即座にプレビュー
- **ファイルツリー & コードビューア** — 生成された各ファイルの内容をシンタックスハイライト付きで確認
- **Agent モード (Tool Call)** — LLM が `read_file` / `write_file` / `apply_edit` / `search_code` などの Tool を使ってファイルを直接操作する高度モード
- **コンソールペイン** — アプリ内およびプレビュー iframe の `console.*` 出力をリアルタイムに確認
- **セッション管理** — 複数のチャットセッションを作成・切り替え・削除、アプリ再起動後も履歴を維持
- **リサイズ可能なパネル** — Artifact ペインをドラッグで幅調整、サイドバーの折りたたみに対応
- **送信キャンセル** — ストリーミング中や Tool Call ループ実行中にいつでも Stop ボタンで中断可能

## 2 つの動作モード

### Markdown モード (デフォルト)

LLM が Markdown コードブロック (`` ```html path=index.html ``) でコードを出力し、アプリが自動的にパースしてファイルに保存・プレビューします。シンプルなプロトタイピングに向きます。

### Agent モード

LLM が Tool Call 機能を使ってワークスペースのファイルを直接操作します。差分編集 (`apply_edit`) による最小変更、既存コードの読み取り (`read_file`)、コード検索 (`search_code`) など、大規模なコード操作や継続的な修正に適しています。

| モード | Artifact 更新 | 適した用途 |
|--------|--------------|-----------|
| Markdown | LLM レスポンス後の自動パース | 新規プロトタイプ作成 |
| Agent | Tool による直接ファイル操作 | 既存コードの修正・リファクタリング |

## 技術スタック

| レイヤー | 技術 |
|---------|------|
| バックエンド | Go 1.23 + [Wails v2](https://wails.io/) |
| フロントエンド | Svelte 5 + TypeScript + Vite 6 |
| LLM プロバイダ | Ollama API / OpenRouter API (SSE ストリーミング) |
| データ永続化 | JSON ファイル (セッション・設定) |
| Artifact サーバー | Go 標準 `net/http` (エフェメラルサーバー、`127.0.0.1` のランダムポート) |

## アーキテクチャ

```
User → Svelte UI → Wails Binding → Go ChatService → Provider (Ollama/OpenRouter)
                                                      ↓ SSE Stream
                                          EventsEmit("llm-event") → Svelte UI 更新
                                          ArtifactManager → ファイル書き出し
                                          ArtifactServer → iframe プレビュー
```

- **Provider インターフェース** (`provider/`) — `Stream()` / `StreamWithTools()` で Ollama / OpenRouter を統一的に扱う
- **ArtifactManager** (`artifacts/`) — セッションごとのワークスペース管理、アトミックなファイル書き込み、コード検索
- **ArtifactServer** (`artifacts/`) — `Cache-Control: no-store` で常に最新ファイルをサーブする HTTP サーバー
- **SessionManager** (`session/`) — UUID ベースのセッション、JSON ファイルで履歴を永続化
- **ToolManager** (`tools/`) — Agent モードの Tool を登録・実行 (30 秒タイムアウト付き)

## プロジェクト構成

```
fm-my-canvas/
├── main.go                  # Wails アプリエントリーポイント
├── app.go                   # アプリ構造体、ライフサイクル
├── chat.go                  # チャットサービス (バインドメソッド、Artifact パース)
├── chat_test.go
├── provider/                # LLM プロバイダ抽象化
│   ├── provider.go          # Provider interface, StreamEvent, ToolDefinition
│   ├── ollama.go            # Ollama SSE クライアント
│   ├── openrouter.go        # OpenRouter SSE クライアント
│   ├── ollama_test.go
│   └── openrouter_test.go
├── artifacts/               # Artifact ファイル管理 & HTTP サーバー
│   ├── manager.go           # ワークスペース管理、アトミック書き込み、パス検証
│   ├── server.go            # エフェメラルプレビューサーバー、console interceptor
│   ├── search.go            # 正規表現コード検索
│   ├── manager_test.go
│   └── search_test.go
├── session/                 # セッション管理 (JSON 永続化)
│   └── manager.go
├── config/                  # 設定管理
│   └── config.go
├── tools/                   # Agent モード Tool 群
│   ├── tool.go              # Tool インターフェース
│   ├── registry.go          # Tool 登録・実行ディスパッチ
│   ├── file_read_tool.go    # read_file
│   ├── file_write_tool.go   # write_file
│   ├── file_list_tool.go    # list_files
│   ├── edit_engine.go       # Search/Replace エンジン
│   ├── edit_apply_tool.go   # apply_edit
│   ├── search_code_tool.go  # search_code
│   └── *_test.go
├── types/                   # 共通型定義
│   ├── types.go             # Role, Message, ToolCall, Session, ArtifactFileInfo
│   └── types_test.go
├── frontend/
│   ├── src/
│   │   ├── App.svelte       # ルート: レイアウト (Sidebar + Chat + Artifact)
│   │   ├── components/
│   │   │   ├── chat/        # ChatArea, ChatInput, ChatMessage, ToolCallMessage
│   │   │   ├── artifacts/   # ArtifactPanel, PreviewPane, CodeEditor, FileTree, ConsolePane
│   │   │   └── layout/      # Sidebar, SettingsModal
│   │   └── lib/
│   │       ├── services/    # Wails バインディング呼び出し
│   │       ├── stores/      # Svelte 5 $state ベースの状態管理
│   │       └── parsers/     # LLM 出力の Artifact パーサー
│   ├── package.json
│   └── vite.config.ts
├── docs/                    # 設計ドキュメント
├── mise.toml                # ツール・タスク定義
└── wails.json               # Wails プロジェクト設定
```

## セットアップ

前提条件: [mise](https://mise.jdx.dev/) がインストールされていること。

```powershell
# ツールのインストール (Go, Node.js)
mise install

# Wails CLI のインストール
mise run setup

# フロントエンド依存パッケージのインストール
mise run frontend:install
```

## 開発

```powershell
# 開発サーバー起動 (Hot Reload)
mise run dev
```

## ビルド

```powershell
# プロダクションビルド
mise run build
```

ビルド成果物は `build/bin/` に生成されます。

## テスト

```powershell
# 全テスト実行
mise run test

# 詳細出力
mise run test:verbose
```

## 設定

アプリ内の Settings 画面から以下を設定できます。設定は `~/.config/fm-my-canvas/config.json` に保存されます。

| 項目 | 説明 | デフォルト |
|------|------|-----------|
| Provider | `ollama` または `openrouter` | `ollama` |
| Ollama Endpoint | Ollama API エンドポイント | `http://localhost:11434` |
| Ollama Model | 使用するモデル名 | `llama3` |
| OpenRouter API Key | OpenRouter の API キー | (空) |
| OpenRouter Model | OpenRouter で使用するモデル | (空) |
| Agent Mode | Tool Call ベースの Agent モードを有効化 | OFF |

## 使い方

1. サイドバーの **+ New** で新しいチャットセッションを作成
2. チャット入力欄にプロンプトを入力し **Ctrl+Enter** または **Send** で送信
3. LLM がコードを出力すると、自動的に Artifact ペインにファイルが表示される
4. Artifact ペインの **Preview** タブでプレビュー、**Code** タブでファイルごとのソースコードを確認、**Console** タブでログ出力を確認
5. 継続的な修正指示でコードをアップデート可能 (セッション内の会話コンテキストを維持)
6. Agent モードを ON にすると、Tool Call による精密なファイル操作が可能

## Agent モードで使用可能な Tools

| Tool | 説明 |
|------|------|
| `read_file(path)` | ワークスペース内のファイルを読み取る |
| `write_file(path, content)` | ワークスペースにファイルを書き込む |
| `list_files([path])` | ワークスペースのファイル一覧を取得 |
| `apply_edit(path, search, replace)` | Search/Replace 方式で部分編集を適用 |
| `search_code(pattern, [file_pattern])` | 正規表現でコードを検索 |

Tool Call ラウンドは 1 メッセージあたり最大 10 回、全体で最大 5 分間実行されます。個別 Tool の実行には 30 秒のタイムアウトが設定されています。

## データの保存先

| データ | パス |
|--------|------|
| 設定 | `~/.config/fm-my-canvas/config.json` |
| セッション履歴 | `~/.config/fm-my-canvas/sessions/<uuid>.json` |
| Artifact ファイル | `~/.config/fm-my-canvas/artifacts/<session-uuid>/` |

## セキュリティ

- 生成されたスクリプトは `sandbox` 属性付きの `<iframe>` 内で実行され、ローカルファイルシステムへの不正アクセスを防止
- Tool によるファイル操作はすべてパス検証によりワークスペース内に制限 (パストラバーサル防止)
- ファイルサイズは読み書きともに最大 1MB に制限
- Artifact サーバーは `127.0.0.1` のランダムポートでのみリッスン

## 設計ドキュメント

- [01. 要求仕様書](docs/01_requirement.md)
- [02. 実装設計仕様書](docs/02_specification.md)
- [03. エージェント拡張設計書](docs/03_agent_update.md)
- [04. Phase 1 実装仕様書](docs/04_agent_specification.md)
- [05. Phase 2 実装仕様書](docs/05_agent_specification_2.md)
- [06. Phase 3 実装仕様書](docs/06_agent_specification_3.md)

## License

Private
