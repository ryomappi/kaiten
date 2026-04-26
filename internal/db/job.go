package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type Job struct {
	ID         string
	Command    string
	Priority   int
	Status     Status
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	ExitCode   *int
	Stdout     string
	Stderr     string
}

func InsertJob(db *sql.DB, id, command string, priority int) error {
	_, err := db.Exec(
		`INSERT INTO jobs (id, command, priority) VALUES (?, ?, ?)`,
		id, command, priority,
	)
	return err
}

func ListJobs(db *sql.DB, status string, limit int) ([]Job, error) {
	q := `SELECT id, command, priority, status, created_at,
		started_at, finished_at, exit_code FROM jobs`
	args := []any{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, limit)
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		var startedAt, finishedAt sql.NullString
		var exitCode sql.NullInt64
		if err := rows.Scan(&j.ID, &j.Command, &j.Priority, &j.Status,
			&j.CreatedAt, &startedAt, &finishedAt, &exitCode); err != nil {
			return nil, err
		}
		if startedAt.Valid {
			t, _ := time.Parse("2006-01-02T15:04:05Z", startedAt.String)
			j.StartedAt = &t
		}
		if finishedAt.Valid {
			t, _ := time.Parse("2006-01-02T15:04:05Z", finishedAt.String)
			j.FinishedAt = &t
		}
		if exitCode.Valid {
			c := int(exitCode.Int64)
			j.ExitCode = &c
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func GetJob(db *sql.DB, id string) (*Job, error) {
	row := db.QueryRow(
		`SELECT id, command, priority, status, created_at,
		started_at, finished_at, exit_code, stdout, stderr FROM jobs WHERE id LIKE ?`, id+"%",
	)
	var j Job
	var startedAt, finishedAt sql.NullString
	var exitCode sql.NullInt64
	if err := row.Scan(&j.ID, &j.Command, &j.Priority, &j.Status,
		&j.CreatedAt, &startedAt, &finishedAt, &exitCode, &j.Stdout, &j.Stderr); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job %s not found", id)
		}
		return nil, err
	}
	if startedAt.Valid {
		t, _ := time.Parse("2006-01-02T15:04:05Z", startedAt.String)
		j.StartedAt = &t
	}
	if finishedAt.Valid {
		t, _ := time.Parse("2006-01-02T15:04:05Z", finishedAt.String)
		j.FinishedAt = &t
	}
	if exitCode.Valid {
		c := int(exitCode.Int64)
		j.ExitCode = &c
	}
	return &j, nil
}

func CancelJob(db *sql.DB, id string) error {
	res, err := db.Exec(
		`UPDATE jobs SET status='cancelled' WHERE id LIKE ? AND status IN ('pending','running')`, id+"%",
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job %s not found or not cancellable", id)
	}
	return nil
}

// ClaimPending atomically claims up to n pending jobs for the worker.
func ClaimPending(db *sql.DB, n int) ([]Job, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(
		`SELECT id, command FROM jobs WHERE status='pending'
		ORDER BY priority DESC, created_at ASC LIMIT ?`, n,
	)
	if err != nil {
		return nil, err
	}
	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Command); err != nil {
			rows.Close()
			return nil, err
		}
		jobs = append(jobs, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, j := range jobs {
		_, err := tx.Exec(
			`UPDATE jobs SET status='running', started_at=datetime('now') WHERE id=? AND status='pending'`,
			j.ID,
		)
		if err != nil {
			return nil, err
		}
	}
	return jobs, tx.Commit()
}

func FinishJob(db *sql.DB, id string, exitCode int, stdout, stderr string) error {
	status := StatusDone
	if exitCode != 0 {
		status = StatusFailed
	}
	_, err := db.Exec(
		`UPDATE jobs SET status=?, exit_code=?, stdout=?, stderr=?,
		finished_at=datetime('now') WHERE id=?`,
		status, exitCode, stdout, stderr, id,
	)
	return err
}

// MarkCancelledRunning marks a running job as cancelled (used when worker detects cancellation).
func MarkCancelledRunning(db *sql.DB, id string) error {
	_, err := db.Exec(
		`UPDATE jobs SET status='cancelled', finished_at=datetime('now') WHERE id=?`, id,
	)
	return err
}
