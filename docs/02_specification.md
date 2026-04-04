# 02. 実装設計仕様書 (Implementation Specification)

## 0. 前提と制約 (AGENTS.md 準拠)

本プロジェクトは既存の Wails v2 プロジェクトをベースに開発を行う。以下の制約を最優先とする。

*   **ツールチェーン**: `wails`, `go`, `node`, `npm` はすべて **mise 経由** で使用する。直接コマンドを叩かない。
*   **タスク実行**: `mise.toml` に定義された `mise run <task>` を最優先する。
*   **バージョン管理**: **jj** のみを使用し、git コマンドは使用しない。
*   **シェル**: Windows + PowerShell を前提とする。
*   **フロントエンド**: 既存の `frontend/` ディレクトリに Svelte5 + TypeScript を導入する（Wails テンプレの再作成は行わない）。

---

## 1. 技術スタックとアーキテクチャ

本アプリは **Wails v2** をベースに、Go をバックエンド、Svelte5 をフロントエンドとした構成とする。

*   **バックエンド (Go)**: LLM との SSE 制御、Artifacts ファイル管理（アトミック書き込み、エフェメラル HTTP サーバ）、ローカルセッション管理と設定ファイルの永続化を担当。
*   **フロントエンド (Svelte5 + TS + Vite)**: チャット UI、設定画面、および Artifacts プレビュー（`<iframe>` サンドボックス）の表示、LLM の出力ストリームの累積表示とシンタックスハイライトを担当。

### 1.1. コンポーネント構成の概要

```mermaid
sequenceDiagram
    User ->> Svelte(Frontend): チャットメッセージ投入
    Svelte ->> Golang(Bound Method): sendMessage(session_id, message)
    Golang ->> OpenRouter/Ollama: API 呼び出し (Streaming)
    loop LLM chunks
        OpenRouter/Ollama ->> Golang: Stream chunk
        Golang ->> ArtifactManager: Chunk の累積 & ファイル一時書き込み
        Golang ->> Svelte: EventsEmit(llm-event, data)
        Svelte ->> Svelte: UI の更新 & ファイルステータス反映
    end
    Golang -- Svelte: ストリーム終了イベント (llm-event, "done")
```

---

## 2. バックエンド設計 (Go)

### 2.1. パッケージ構成

Wails v2 の標準構成に従い、Go パッケージはプロジェクトルートに配置する。機能ごとにファイルを分割し、関心を分離する。

```text
fm-my-canvas/
├── main.go             # Wails アプリエントリーポイント
├── app.go              # アプリ構造体、ライフサイクルメソッド
├── chat.go             # チャットサービス（LLM プロバイダ呼び出し）
├── provider/           # LLM API クライアント（ollama.go, openrouter.go）
├── artifacts/          # ファイル管理 & HTTP サーバー管理
├── session/            # チャット履歴とセッション管理（JSON ファイル）
├── config/             # API キーや設定情報の管理
└── types/              # 共通型定義
```

### 2.2. LLM プロバイダの抽象化 (`provider/`)

Ollama と OpenRouter のストリーミング API を統一的に扱える設計。

*   `type Provider interface { Stream(ctx context.Context, prompt string, history []Message, cb func(chunk string)) error }` を用意し、各実装を提供。
*   `chat.go` のバインドメソッド (`SendMessage`) はこのインターフェースのみを参照し、設定に応じて Ollama クライアントか OpenRouter クライアントに振り分ける。
*   **Chunk 処理**: ストリーム受信時にコールバックを呼ぶ際、Wails `runtime.EventsEmit` を挟みフロントに `llm-event`（chunk 追加、ファイル作成開始、ファイル終了通知など）として通知。

### 2.3. Artifact ファイルマネージャ & エフェメラル HTTP サーバ (`artifacts/`)

*   **ワークスペースの作成**: セッションが開始する際、アプリケーションのデータディレクトリ内に `wails.UserHomeDir() + "/fm-my-canvas/sessions/<session-uuid>"` のようなパスで専用ディレクトリを作成。
*   **アトミックな書き込み**: ファイル更新中（LLM 出力中など）に一時的な `<filename>.tmp` を配置し、書き込み完了後に `os.Rename` で `index.html` 等へ切り替える。これによりフロントが書きかけのファイルを取得する現象を防ぐ。
*   **エフェメラルサーバー**:
    *   初回のファイル書き込みが発生した時点（またはセッション開始直前）で、`net.Listen("127.0.0.1:0")` によってカーネルにポートを割り振る。
    *   `http.ServeMux` で `http.FileServer` を立ち上げ、`http://127.0.0.1:<port>/` で静的ファイルをサーブさせる。
    *   `Cache-Control: no-cache, no-store` ヘッダを挿入し、LLM によって動的に変更されるファイルが常に更新されるようブラウザキャッシュを抑制する。
    *   Wails の `ctx.Context` を使用し、アプリケーション終了時に `server.Shutdown()` を確実に実行する。
    *   サーバー URL は起動完了後、Wails `EventsEmit` でフロントに通知 ("artifact-server-ready")。

### 2.4. ローカルセッション管理 (`session/`)

*   チャットの `prompt` と `response` を履歴オブジェクトとしてセッション単位で JSON ファイルに格納。
*   初回生成後、LLM が吐き出した最終的なファイル構成をセッションとひも付け、次回読み込み時に前回の状態を復帰可能にする。
*   **MVP では JSON ファイル方式を採用**し、依存の増加とセットアップコストを抑える。

---

## 3. フロントエンド設計 (TypeScript + Svelte5)

### 3.1. Svelte の導入と構成

既存の `frontend/` ディレクトリに Svelte5 を追加導入する。

*   **依存パッケージの追加**: `mise run setup` または `mise run frontend:install` で `svelte`, `@sveltejs/vite-plugin-svelte` をインストール。
*   **Vite 設定の更新**: `vite.config.ts` に Svelte プラグインを登録。
*   **tsconfig.json の更新**: Svelte の型定義を認識させる。

```text
frontend/
├── src/
│   ├── lib/
│   │   ├── services/       # Go とのブリッジ関数（Wails Runtime API の呼び出し）
│   │   ├── stores/         # $state グローバル状態（セッション情報、ユーザー設定、イベントストリーム管理）
│   │   └── parsers/        # LLM 応答のパース（マルチファイル抽出ロジック）
│   ├── components/
│   │   ├── chat/           # ChatMessage.svelte, ChatInput.svelte, ChatHistoryList.svelte
│   │   ├── artifacts/      # CodeEditor.svelte, PreviewPane.svelte (iframe), FileTree.svelte
│   │   └── layout/         # ResizablePanels.svelte, Sidebar.svelte, MainArea.svelte
│   ├── App.svelte          # ルートコンポーネント
│   ├── main.ts             # エントリーポイント
│   └── vite-env.d.ts
├── index.html
├── package.json
├── tsconfig.json
└── vite.config.ts
```

### 3.2. 状態管理と Wails バインディング

*   **Stores**: `let sessions = $state([]); let currentSession = $state(null); let currentMessage = $state('')` など、Svelte 5 の `$state` ルートでスコープの限定されたリアクティブなステートを管理。グローバルな状態は専用の Store モジュールにまとめておく。
*   **Go Binding**: Wails CLI が生成する `wailsjs/go/**/*.ts` 関数をインポート。`go.chat.SendMessage(session, message)` のように呼び出し。
*   **Real-time Events**: `import { EventsOn, EventsOff } from "@wails/runtime/runtime"` を使用して、`EventsOn("llm-event", callback)` で Go からのストリーム受信ハンドラを登録する。

### 3.3. Artifact 出力解析と UI 表示

LLM は通常、応答に Markdown 形式のコードブロックを出力する。フロント側でそれを解析する必要がある（LLM がストリーミング中に出力されるため逐次解析を行う）。

1.  **パーサー**: フロントの `parsers` モジュールに、テキストを監視し ````html (path: index.html)... ```` などのパターンを正規表現で切り分けて、対応する Go の関数やファイルマネージャの状態にマップする仕組み。パス名はユーザー/LLM 間の取り決めで `index.html` や `main.css` を識別。
2.  **シンタックス表示**: `highlight.js` または `shiki` などのライブラリを `CodeEditor` コンポーネント内で利用し、パース中のコードブロックもリアルタイムに色付けして可読性を向上する。

### 3.4. プレビュー機能（Iframe サンドボックス）

Svelte の `PreviewPane` コンポーネントにて、Go 側から通知された `http://127.0.0.1:<port>/` を `<iframe>` の `src` に設定する。

```html
<!-- PreviewPane.svelte 例 -->
<script lang="ts">
  let { previewUrl = '', key = '' }: { previewUrl: string, key: string };
</script>

<iframe
  class="sandbox-iframe"
  src={previewUrl}
  title="Artifact Sandbox"
  sandbox="allow-scripts allow-forms allow-modals"
/>
```

*   ファイルが更新されても、`http.FileServer` は自動的に最新のファイルを返すため、ブラウザがキャッシュを無視する限りリロードせずとも表示は変わらない。Svelte 側は `key` 属性をバインドしたりする手もある。

---

## 4. プロジェクト全体のディレクトリ構成

```text
fm-my-canvas/
├── main.go                 # Wails アプリ起動設定 (Options 等)
├── app.go                  # アプリ構造体、ライフサイクル
├── chat.go                 # チャットサービス（バインドメソッド）
├── provider/               # LLM プロバイダ実装
├── artifacts/              # アーティファクト管理
├── session/                # セッション管理
├── config/                 # 設定管理
├── types/                  # 共通型定義
├── go.mod
├── go.sum
├── wails.json
├── mise.toml               # ツール・タスク定義
├── frontend/
│   ├── src/
│   │   ├── lib/
│   │   ├── components/
│   │   ├── App.svelte
│   │   └── main.ts
│   ├── index.html
│   └── package.json
├── build/
└── docs/
    ├── 01_requirement.md
    └── 02_specification.md
```

---

## 5. 主要な開発手順 (MVP までのタスク切り出し)

### 5.1. セットアップ
1.  **Svelte の導入**: `mise run frontend:install-svelte` (新規タスク定義) で Svelte5 + Vite プラグインをインストール。
2.  **Vite 設定の更新**: `vite.config.ts` に Svelte プラグインを登録、`tsconfig.json` を更新。
3.  **動作確認**: `mise run dev` で Wails 開発サーバーが起動し、Svelte コンポーネントが描画されることを確認。

### 5.2. 基本機能実装
4.  **基本 UI とルーティングの構築**: プロジェクトのルートに Sidebar と Chat/Preview ページを作成する。Go と TS の接続が通ることを確認する。
5.  **LLM クライアントの実装**: 先に Ollama を固定でターゲットにし、ローカルのモデルでチャットとストリーミングができる状態を完成させる。
6.  **ファイルパーサーと Artifacts 表示**: LLM からのマークダウン出力を構造化し、CodeEditor にファイル構成を表示、Preview に読み込むパイプを完成させる。
7.  **セッションの永続化**: アプリ再起動でも履歴が消えないよう JSON ファイルに保存。
8.  **OpenRouter 対応**: プロバイダを切り替え設定できるよう拡張し、エクスポート機能を追加。

### 5.3. 開発コマンド (mise task)

```powershell
# 開発サーバー起動
mise run dev

# ビルド
mise run build

# フロントエンド依存パッケージインストール
mise run frontend:install

# Svelte 追加インストール（新規定義）
mise run frontend:install-svelte
```
