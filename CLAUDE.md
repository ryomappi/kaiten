# Kaiten — 設計・実装ガイド

## 概要

シェルコマンドをジョブとしてキューに登録・管理・実行するGo製CLIツール。
単一バイナリで、`worker` サブコマンドがデーモンとして動作し、その他のサブコマンドがCLIクライアントとして機能する。

## 依存ライブラリ

```
github.com/spf13/cobra       CLIフレームワーク
modernc.org/sqlite           SQLiteドライバー（CGO不要、純Go実装）
github.com/google/uuid       ジョブID（UUID v4）生成
```

## ディレクトリ構成

```
kaiten/
├── main.go                  エントリーポイント。cmd.Execute() を呼ぶだけ
├── cmd/
│   ├── root.go              cobraルートコマンド。--db グローバルフラグを定義
│   ├── worker.go            kaiten worker サブコマンド
│   ├── enqueue.go           kaiten enqueue サブコマンド
│   ├── list.go              kaiten list サブコマンド
│   ├── cancel.go            kaiten cancel サブコマンド
│   └── logs.go              kaiten logs サブコマンド
└── internal/
    ├── db/
    │   ├── db.go            DB接続・初期化（Open関数）
    │   ├── schema.go        CREATE TABLE / CREATE INDEX（migrate関数）
    │   └── job.go           ジョブのCRUD操作
    └── worker/
        └── worker.go        ポーリングループと並列実行ロジック
```

## アーキテクチャ

CLIクライアントとworkerデーモンが**同じSQLiteファイルを直接共有**する。IPCやHTTP APIは持たない。

```
kaiten enqueue / list / cancel / logs  ←→  ~/.kaiten/jobs.db  ←→  kaiten worker
```

- CLIはジョブの挿入・参照・キャンセルのみ行う
- workerはポーリングでpendingジョブを取得して実行し、結果をDBに書き戻す
- デフォルトDB: `~/.kaiten/jobs.db`。`--db` フラグで変更可能

## DBスキーマ

```sql
CREATE TABLE IF NOT EXISTS jobs (
  id          TEXT PRIMARY KEY,
  command     TEXT NOT NULL,
  priority    INTEGER NOT NULL DEFAULT 0,
  status      TEXT NOT NULL DEFAULT 'pending',
  created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
  started_at  DATETIME,
  finished_at DATETIME,
  exit_code   INTEGER,
  stdout      TEXT,
  stderr      TEXT
);
CREATE INDEX IF NOT EXISTS idx_status_priority ON jobs(status, priority DESC, created_at ASC);
```

ステータス遷移: `pending` → `running` → `done` / `failed` / `cancelled`

## SQLite接続設定

```go
sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000&_fk=true")
db.SetMaxOpenConns(1)
```

- **WALモード**: workerとCLIが同時にアクセスするため必須
- **MaxOpenConns(1)**: SQLiteはマルチライタ非対応のため接続を1本に制限
- **_timeout=5000**: ロック競合時に最大5秒待機してからエラー

## ジョブIDとプレフィックスマッチ

ジョブIDはUUID v4（例: `a662e277-c71e-4fef-ac4e-813b9f2e3748`）。
`list` コマンドの出力では先頭8文字のみ表示する。
`logs` と `cancel` はプレフィックスマッチ（`WHERE id LIKE ?` に `id + "%"` を渡す）で短縮IDを受け付ける。

## workerの並列制御

```go
sem := make(chan struct{}, w.Workers)  // セマフォチャネル

// ジョブ取得: 空きスロット分だけDBから取得
free := w.Workers - len(sem)
jobs, _ := db.ClaimPending(w.DB, free)

// goroutine起動
for _, j := range jobs {
    sem <- struct{}{}   // スロット確保（満杯なら待機）
    wg.Add(1)
    go func(id, command string) {
        defer func() { <-sem; wg.Done() }()  // スロット解放
        w.execute(ctx, id, command)
    }(j.ID, j.Command)
}
```

## ClaimPendingのトランザクション

workerが複数起動することを想定し、`SELECT` と `UPDATE` をトランザクションで囲む。
`UPDATE ... WHERE id=? AND status='pending'` の条件により、他のworkerが先にclaimした場合は安全にスキップされる。

```go
tx.Query(`SELECT id, command FROM jobs WHERE status='pending'
          ORDER BY priority DESC, created_at ASC LIMIT ?`, n)
// → 取得したIDに対して
tx.Exec(`UPDATE jobs SET status='running', started_at=datetime('now')
         WHERE id=? AND status='pending'`, j.ID)
tx.Commit()
```

## Graceful Shutdown

`signal.NotifyContext` でSIGINT/SIGTERMを受け取り、Contextをキャンセルする。
workerのRunループはCtx.Done()で抜け、`wg.Wait()` で実行中のgoroutineの終了を待ってから停止する。

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()
w.Run(ctx)  // シグナル受信後、実行中ジョブが完了してから返る
```

実行中ジョブは `exec.CommandContext(ctx, ...)` を使っているため、ctx cancel時にプロセスもKillされ、ステータスが `cancelled` に更新される。

## コマンドの解析

コマンド文字列は `strings.Fields` でスペース区切りに分割して `exec.Command` に渡す。
シェル経由では実行しないため、シェル記法（パイプ、リダイレクト等）は使えない。
シェル記法が必要な場合は `bash -c "..."` をコマンドとして登録する。

```bash
kaiten enqueue -- bash -c "cat file.txt | grep foo > out.txt"
```

## 優先度

`priority` は整数で、大きいほど優先度が高い（デフォルト0）。
同一優先度の場合は `created_at ASC`（FIFO）で実行順が決まる。

## list出力のカラー表示

ターミナルのANSIエスケープで色付け（外部ライブラリなし）:
- `running` → 黄 `\033[33m`
- `done` → 緑 `\033[32m`
- `failed` → 赤 `\033[31m`
- `cancelled` → グレー `\033[90m`
