# Kaiten

Kaiten is a lightweight CLI tool to manage job queues for shell command execution.

## Architecture

```
kaiten enqueue / list / cancel / logs
        ↓  (SQLite R/W)
   ~/.kaiten/jobs.db
        ↑  (polling + execution)
kaiten worker
```

The worker daemon and CLI client share the same SQLite database directly — no separate server or IPC required.

## Installation

### curl (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/ryomappi/kaiten/main/install.sh | sh
```

To install to a custom directory:

```bash
INSTALL_DIR=$HOME/.local/bin curl -fsSL https://raw.githubusercontent.com/ryomappi/kaiten/main/install.sh | sh
```

### go install

```bash
go install github.com/ryomappi/kaiten@latest
```

### Build from source

```bash
git clone https://github.com/ryomappi/kaiten
cd kaiten
make install   # installs to ~/.local/bin
```

## Commands

### `kaiten worker`

Start the worker daemon. Polls the queue and executes jobs in parallel.

```bash
kaiten worker [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--workers` | `4` | Number of parallel workers |
| `--poll` | `1s` | Polling interval (e.g. `500ms`, `2s`) |

### `kaiten enqueue`

Add a shell command to the queue. Prints the job ID on success.

```bash
kaiten enqueue [flags] -- <command> [args...]
```

| Flag | Default | Description |
|---|---|---|
| `--priority`, `-p` | `0` | Job priority (higher = runs first) |

### `kaiten list`

List jobs.

```bash
kaiten list [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--status`, `-s` | `` | Filter by status: `pending`, `running`, `done`, `failed`, `cancelled` |
| `--limit`, `-n` | `50` | Max number of jobs to show |

### `kaiten logs`

Show stdout/stderr of a completed job.

```bash
kaiten logs <job-id>
```

### `kaiten cancel`

Cancel a pending or running job.

```bash
kaiten cancel <job-id>
```

> Job IDs can be abbreviated to their first 8 characters (as shown in `kaiten list`).

## Global Flags

| Flag | Default | Description |
|---|---|---|
| `--db` | `~/.kaiten/jobs.db` | Path to the SQLite database file |

## Example

```bash
# Terminal 1: start the worker
kaiten worker --workers 3

# Terminal 2: enqueue jobs
kaiten enqueue --priority 10 -- python3 train.py
kaiten enqueue --priority 5  -- python3 evaluate.py
kaiten enqueue               -- echo "done"

# Check status
kaiten list

# View output (short ID works)
kaiten logs a662e277

# Cancel a job
kaiten cancel a662e277
```

## Job Statuses

| Status | Description |
|---|---|
| `pending` | Waiting to be picked up by a worker |
| `running` | Currently executing |
| `done` | Finished with exit code 0 |
| `failed` | Finished with non-zero exit code |
| `cancelled` | Cancelled before or during execution |
