# zmxx

`zmxx` は [git worktree](https://git-scm.com/docs/git-worktree) と
[zmx](https://zmx.sh) を組み合わせたワークスペースツールです。

1. ターミナルで作業対象の Git リポジトリに移動する
2. `zmxx new <branch>` で worktree を作成し、zmx セッション内で neovim を起動する
3. neovim から `Zl` で zmx セッションのピッカーを開き、別のワークスペースへ切り替える

## 要件

- Go 1.24 以上（ビルド時）
- [zmx](https://zmx.sh) 0.8.0 以上（ラベルとセッション切替に必要）
- Neovim 0.10 以上（`vim.system` を使用）
- [fzf-lua](https://github.com/ibhagwan/fzf-lua)（ピッカー UI）

## インストール

### CLI

```sh
go install github.com/devoc09/zmxx/cmd/zmxx@latest
# またはローカルでビルド
go build -o ~/.local/bin/zmxx ./cmd/zmxx
```

### Neovim プラグイン（lazy.nvim）

```lua
{
  "devoc09/zmxx",
  dependencies = { "ibhagwan/fzf-lua" },
  event = "VeryLazy",
}
```

## 使い方

```sh
# ワークスペースを作成して neovim を起動（既存ブランチなら再利用、なければ HEAD から新規作成）
zmxx new feature/foo

# 別の基点からブランチを作る
zmxx new feature/bar --base main

# このリポジトリの管理済みワークスペース一覧
zmxx list

# セッションを kill して worktree を削除（ブランチは残る）
zmxx remove feature/foo

# 作業中（--force で確認をスキップし、未コミット変更も破棄）
zmxx remove feature/foo --force
```

Neovim 内では:

| 操作 | 説明 |
| --- | --- |
| `Zl` | 全 zmxx セッションのピッカーを開く |
| `:ZmxxSessions` | ピッカーを開く（コマンド版） |

ピッカーでは:

- セッション名は非表示で、リポジトリ名・ブランチ・worktree パス・接続数で絞り込み
- 選択中のセッションのスクロールバックを `zmx history` でプレビュー（`follow` で下端追従）
- `Enter` で端末の zmx クライアントを選択したセッションへ切り替え（neovim 自体は元のセッションで動作継続）

## 仕組み

- worktree は `$XDG_DATA_HOME/zmxx/worktrees/<repo-id>/<branch-slug>-<hash>` に決定的なパスで配置（未設定時は `~/.local/share/zmxx`）
- `<repo-id>` は origin URL（無ければ git common dir）の SHA-256 先頭 12 桁
- zmx セッション名は unix socket の長さ制限に収まる `zmxx-<repo-id>-<branch-hash>`（各 12 桁）
- セッションには `zmxx=1` / `zmxx.repo` / `zmxx.reponame` / `zmxx.branch` / `zmxx.worktree` のラベルを付与（値は base64url）。ピッカーはこのラベルで全 zmxx セッションを検出
- 切り替えは zmx のネイティブな Switch IPC を利用: neovim から `zmxx switch <name>` → `zmx attach <name>` を実行すると、`ZMX_SESSION` を手がかりに zmx が**端末のクライアントだけ**を移動させます
- `zmxx switch` は存在するセッションのみ受け付け（upsert による誤生成を防止）

## 制約・メモ

- `ZMX_SESSION_PREFIX` は zmxx が管理するセッションでは使わないでください（zmxx は全 zmx 呼び出しで prefix を空にします）
- `zmxx remove` は最初に zmx セッションを kill します。自分がアタッチ中のセッションを削除すると端末の接続が切れます
- セッションはリポジトリをまたいで 1 つの名前空間で管理されます（ピッカーは全 zmxx セッションを表示）
- ワークスペース内で neovim 以外のコマンドを起動したい場合は `zmx attach <session-name> <command>` を直接利用できます（`zmxx list` でセッション名を確認）
