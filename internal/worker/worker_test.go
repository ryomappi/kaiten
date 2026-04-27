package worker

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ryomappi/kaiten/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func newWorker(d *sql.DB) *Worker {
	return &Worker{
		DB:       d,
		Workers:  4,
		PollFreq: 50 * time.Millisecond,
	}
}

func waitStatus(t *testing.T, d *sql.DB, id string, want db.Status) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		j, err := db.GetJob(d, id)
		if err == nil && j.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	j, _ := db.GetJob(d, id)
	var got db.Status
	if j != nil {
		got = j.Status
	}
	t.Errorf("job %s: status = %q, want %q (timed out)", id, got, want)
}

func TestExecute_Success(t *testing.T) {
	d := openTestDB(t)
	_ = db.InsertJob(d, "job-1", "echo hello", 0)
	jobs, _ := db.ClaimPending(d, 1)

	ctx := context.Background()
	w := newWorker(d)
	w.execute(ctx, jobs[0].ID, jobs[0].Command)

	j, err := db.GetJob(d, "job-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if j.Status != db.StatusDone {
		t.Errorf("status = %q, want done", j.Status)
	}
	if j.Stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", j.Stdout, "hello\n")
	}
}

func TestExecute_Failure(t *testing.T) {
	d := openTestDB(t)
	_ = db.InsertJob(d, "job-1", "false", 0)
	jobs, _ := db.ClaimPending(d, 1)

	ctx := context.Background()
	w := newWorker(d)
	w.execute(ctx, jobs[0].ID, jobs[0].Command)

	j, _ := db.GetJob(d, "job-1")
	if j.Status != db.StatusFailed {
		t.Errorf("status = %q, want failed", j.Status)
	}
	if j.ExitCode == nil || *j.ExitCode == 0 {
		t.Errorf("expected non-zero exit code")
	}
}

func TestExecute_EmptyCommand(t *testing.T) {
	d := openTestDB(t)
	_ = db.InsertJob(d, "job-1", "", 0)
	jobs, _ := db.ClaimPending(d, 1)

	ctx := context.Background()
	w := newWorker(d)
	w.execute(ctx, jobs[0].ID, jobs[0].Command)

	j, _ := db.GetJob(d, "job-1")
	if j.Status != db.StatusFailed {
		t.Errorf("status = %q, want failed", j.Status)
	}
}

func TestExecute_ContextCancel(t *testing.T) {
	d := openTestDB(t)
	_ = db.InsertJob(d, "job-1", "sleep 10", 0)
	jobs, _ := db.ClaimPending(d, 1)

	ctx, cancel := context.WithCancel(context.Background())
	w := newWorker(d)

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.execute(ctx, jobs[0].ID, jobs[0].Command)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	j, _ := db.GetJob(d, "job-1")
	if j.Status != db.StatusCancelled {
		t.Errorf("status = %q, want cancelled", j.Status)
	}
}

func TestRun_GracefulShutdown(t *testing.T) {
	d := openTestDB(t)
	_ = db.InsertJob(d, "job-1", "sleep 0.1", 0)

	ctx, cancel := context.WithCancel(context.Background())
	w := newWorker(d)

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()

	waitStatus(t, d, "job-1", db.StatusRunning)
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	j, _ := db.GetJob(d, "job-1")
	if j.Status != db.StatusDone && j.Status != db.StatusCancelled {
		t.Errorf("status = %q, want done or cancelled", j.Status)
	}
}

func TestRun_Concurrency(t *testing.T) {
	d := openTestDB(t)
	_ = db.InsertJob(d, "job-1", "sleep 0.2", 0)
	_ = db.InsertJob(d, "job-2", "sleep 0.2", 0)

	w := &Worker{DB: d, Workers: 2, PollFreq: 50 * time.Millisecond}

	var running atomic.Int32
	go w.Run(t.Context())

	// wait until both jobs are running concurrently
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		jobs, _ := db.ListJobs(d, "running", 0)
		running.Store(int32(len(jobs)))
		if running.Load() == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if running.Load() != 2 {
		t.Errorf("concurrent running jobs = %d, want 2", running.Load())
	}
}
