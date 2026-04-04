# fm-my-canvas

LLM Chat で Artifact ペインを確認しながら HTML/CSS/JS のプロトタイプを作成できるデスクトップアプリケーション。

## 特徴

- **LLM チャットインターフェース**: Ollama (ローカル) または OpenRouter (クラウド) に対応したストリーミングチャット
- **リアルタイム Artifact プレビュー**: LLM が生成した HTML/CSS/JS を即座にプレビュー
- **ファイルツリー & コードビューア**: 生成された各ファイルの内容を確認可能
- **コンソールペイン**: アプリ内のログ出力をリアルタイムに確認
- **セッション管理**: 複数のチャットセッションを作成・切り替え・削除、永続化対応
- **リサイズ可能なパネル**: Artifact ペインをドラッグで幅調整、サイドバーの折りたたみに対応

## 技術スタック

| レイヤー | 技術 |
|---------|------|
| バックエンド | Go 1.23 + Wails v2 |
| フロントエンド | Svelte 5 + TypeScript + Vite 6 |
| LLM プロバイダ | Ollama API / OpenRouter API (SSE ストリーミング) |
| データ永続化 | JSON ファイル (セッション・設定) |
| Artifact サーバー | Go 標準 `net/http` (エフェメラルサーバー、`127.0.0.1` のランダムポート) |

## アーキテクチャ概要

```
User → Svelte UI → Wails Binding → Go ChatService → Provider (Ollama/OpenRouter)
                                                      ↓ SSE Stream
                                          EventsEmit("llm-event") → Svelte UI 更新
                                          ArtifactManager → ファイル書き出し
                                          ArtifactServer → iframe プレビュー
```

- **Provider インターフェース** (`provider/`): `Stream(ctx, messages, cb)` で統一的にストリーミング受信
- **ArtifactManager** (`artifacts/`): セッションごとのワークスペース管理、アトミックなファイル書き込み (`.tmp` → `rename`)
- **ArtifactServer** (`artifacts/`): `Cache-Control: no-store` で常に最新ファイルをサーブする HTTP サーバー
- **SessionManager** (`session/`): UUID ベースのセッション、JSON ファイルで履歴を永続化
- **ArtifactParser** (フロントエンド): LLM 出力の ````html path=index.html` 形式のコードブロックをリアルタイムにパース

## プロジェクト構成

```
fm-my-canvas/
├── main.go                  # Wails アプリエントリーポイント
├── app.go                   # アプリ構造体、ライフサイクル
├── chat.go                  # チャットサービス (バインドメソッド、Artifact パース)
├── provider/                # LLM プロバイダ抽象化
│   ├── provider.go          # Provider interface
│   ├── ollama.go            # Ollama SSE クライアント
│   └── openrouter.go        # OpenRouter SSE クライアント
├── artifacts/               # Artifact ファイル管理 & HTTP サーバー
│   ├── manager.go           # ワークスペース管理、アトミック書き込み
│   └── server.go            # エフェメラルプレビューサーバー
├── session/                 # セッション管理 (JSON 永続化)
│   └── manager.go
├── config/                  # 設定管理
│   └── config.go
├── types/                   # 共通型定義 (Role, Message, Session)
│   └── types.go
├── frontend/
│   ├── src/
│   │   ├── App.svelte       # ルート: レイアウト (Sidebar + Chat + Artifact)
│   │   ├── components/
│   │   │   ├── chat/        # ChatArea, ChatInput, ChatMessage
│   │   │   ├── artifacts/   # ArtifactPanel, PreviewPane, CodeEditor, FileTree, ConsolePane
│   │   │   └── layout/      # Sidebar, SettingsModal
│   │   └── lib/
│   │       ├── services/    # Wails バインディング呼び出し
│   │       ├── stores/      # Svelte 5 $state ベースの状態管理
│   │       └── parsers/     # LLM 出力の Artifact パーサー
│   ├── package.json
│   └── vite.config.ts
├── docs/
│   ├── 01_requirement.md    # 要求仕様書
│   └── 02_specification.md  # 実装設計仕様書
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

## 設定

アプリ内の Settings 画面から以下を設定できます。設定は `~/.config/fm-my-canvas/config.json` に保存されます。

| 項目 | 説明 | デフォルト |
|------|------|-----------|
| Provider | `ollama` または `openrouter` | `ollama` |
| Ollama Endpoint | Ollama API エンドポイント | `http://localhost:11434` |
| Ollama Model | 使用するモデル名 | `llama3` |
| OpenRouter API Key | OpenRouter の API キー | (空) |
| OpenRouter Model | OpenRouter で使用するモデル | (空) |

## 使い方

1. サイドバーの **+ New** で新しいチャットセッションを作成
2. チャット入力欄にプロンプトを入力し **Ctrl+Enter** または **Send** で送信
3. LLM がコードブロックを出力すると、自動的に Artifact ペインにファイルが表示される
4. Artifact ペインの **Preview** タブでプレビュー、**Code** タブでファイルごとのソースコードを確認
5. 継続的な修正指示でコードをアップデート可能 (セッション内の会話コンテキストを維持)

## データの保存先

- 設定: `~/.config/fm-my-canvas/config.json`
- セッション履歴: `~/.config/fm-my-canvas/sessions/<uuid>.json`
- Artifact ファイル: `~/.config/fm-my-canvas/artifacts/<session-uuid>/`
