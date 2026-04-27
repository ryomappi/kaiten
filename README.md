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
| `--retain-done` | `14` | Days to retain `done` jobs (`0` = keep forever) |
| `--retain-failed` | `30` | Days to retain `failed` jobs (`0` = keep forever) |
| `--retain-cancelled` | `7` | Days to retain `cancelled` jobs (`0` = keep forever) |

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

## Deploying on Linux (systemd)

Create a systemd user service so `kaiten worker` starts automatically on login and restarts on crash.

**1. Create the unit file**

```bash
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/kaiten.service << 'EOF'
[Unit]
Description=Kaiten job queue worker
After=default.target

[Service]
ExecStart=%h/.local/bin/kaiten worker --workers 4
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=default.target
EOF
```

Adjust `ExecStart` if `kaiten` is installed elsewhere, and tune `--workers` as needed.

**2. Enable and start**

```bash
systemctl --user daemon-reload
systemctl --user enable --now kaiten
```

**3. Common operations**

```bash
systemctl --user status kaiten      # check status
systemctl --user stop kaiten        # stop
systemctl --user restart kaiten     # restart
journalctl --user -u kaiten -f      # tail logs
```

**4. Start on boot without login (optional)**

By default, systemd user services only run while the user is logged in. To keep the worker running even after logout (e.g. on a headless server):

```bash
sudo loginctl enable-linger $USER
```

## Job Statuses

| Status | Description |
|---|---|
| `pending` | Waiting to be picked up by a worker |
| `running` | Currently executing |
| `done` | Finished with exit code 0 |
| `failed` | Finished with non-zero exit code |
| `cancelled` | Cancelled before or during execution |
