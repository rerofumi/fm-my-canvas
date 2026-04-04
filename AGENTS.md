## プロジェクト概要

- wails アプリの開発プロジェクト
- プロジェクトツールは mise で管理しているので wails, go, node.js の様なツールは mise 経由で利用する
- mise task による実行を優先する
- リポジトリは jj で管理している、直接 git を操作しない
- 開発は Windows + powershell 上で行われている、シェルコマンドは powershell を前提とし、優先する

---

## ツール使用の優先順位

### 1. パッケージ・依存関係管理
- **Go**: `go.mod` / `go get` を**mise 経由**で使用
- **Node.js**: `package.json` が存在する場合、**mise 経由の npm/pnpm** を使用（直接インストールした npm より mise 優先）

### 2. タスク実行
- **mise task** が定義されている場合、常に `mise run <task>` を優先して使用
- `Makefile` や `package.json` の scripts は mise task の代替として使用しない（mise task にラップする）
- mise task がない場合のみ、ネイティブコマンド（`go run`, `npm run` 等）を使用

### 3. リポジトリ管理（重要）
- **jj** を唯一のバージョン管理ツールとして使用
- **git コマンドは使用しない**（`git status`, `git add`, `git commit` など禁止）
- 常に jj 相当のコマンドを使用:
  - `git status` → `jj status`
  - `git add` → `jj st`（自動トラッキング）または `jj file track`
  - `git commit` → `jj commit` または `jj describe` + `jj bookmark`
  - `git push` → `jj git push`
  - `git pull` → `jj git fetch` + `jj rebase` または `jj new`
- origin remote 操作は `jj git remote` 経由で行う
- jj コマンド使用時は `--no-pager` を付ける

### 4. ファイル・ディレクトリ操作
- **eza** を使用してディレクトリ構造やファイル位置を調べる（`ls` より優先）
- ファイル作成やコマンド実行前に常に `pwd` を実行し、カレントディレクトリの位置を確認
- rg(ripgrep) がインストールされているので検索にこれを用いる事もできる

---

## コーディング規約

### Go (バックエンド)
- 標準の Go フォーマット (`gofmt`, `goimports`) に従う
- 実装最後は `wails run build` でビルド完了することまで確認する
- エラーハンドリングは明示的に行い、`_` による無視は最小限に

### テスト (共通)
- テストが失敗した場合は**実装コード側を修正**し、テストコードは修正しない
- テストコード自体に問題があり修正が必要な場合は、**ユーザーに確認を取ってから**修正する

### TypeScript/JavaScript (フロントエンド)
- プロジェクトに eslint/prettier 設定があれば従う
- Wails バインディングの型定義を活用
- 実装後に eslint による静的解析を必ず行う

#### Svelte 5 特有の注意点
- **コンポーネントのマウント**: `new Component()` は使用不可。`import { mount } from 'svelte'` を使用し `mount(Component, { target })` でマウントする
- **状態管理**: `let count = $state(0)` のように `$state()` ルーンを使用する
- **アセットのインポート**: Svelte 内の画像は `import logo from './assets/image.png'` で Vite 経由でインポートする（`src` 属性に直接パスを書かない）
- **イベント修飾子**: `@click.prevent` 等の修飾子は使用不可。`onclick={(e) => { e.preventDefault(); handler() }}` と記述する

#### Vite 6 特有の注意点
- **ESM 必須**: `package.json` に `"type": "module"` が必要
- **vite.config.ts**: ESM 形式で記述（CommonJS は非対応）

---

## 開発ワークフロー

### セットアップ
```powershell
# 初回セットアップは mise task を使用
mise run setup
```

### ビルド
```powershell
# wails build は mise 経由で実行
mise exec -- wails build
# または mise task が定義されていれば:
mise run build
```

### 変更の保存と共有（jj ワークフロー）
```powershell
# 1. 変更内容の確認
jj status

# 2. コミット（変更の記録）
jj commit -m "説明"

# 3. 新しい実装フェーズに移行するときは new で新しい jj ノードに移る
jj new -m "説明"

# 4. リモートへのプッシュ
jj git push
```

---

## 禁止事項

1. **直接の git コマンド使用**（`git add`, `git commit`, `git push` など）
2. **直接の wails/go/node/npm コマンド**（mise 経由で使用）
3. **ls によるディレクトリ確認**（eza を使用）
4. **ファイル操作前の pwd 確認スキップ**
