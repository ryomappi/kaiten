package db

import (
	"database/sql"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestInsertAndGetJob(t *testing.T) {
	d := openTestDB(t)
	if err := InsertJob(d, "job-1", "echo hello", 0); err != nil {
		t.Fatalf("insert: %v", err)
	}
	j, err := GetJob(d, "job-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if j.Command != "echo hello" {
		t.Errorf("command = %q, want %q", j.Command, "echo hello")
	}
	if j.Status != StatusPending {
		t.Errorf("status = %q, want %q", j.Status, StatusPending)
	}
}

func TestGetJobPrefixMatch(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "abcdef12-0000-0000-0000-000000000000", "echo hi", 0)
	j, err := GetJob(d, "abcdef12")
	if err != nil {
		t.Fatalf("prefix match failed: %v", err)
	}
	if j.ID != "abcdef12-0000-0000-0000-000000000000" {
		t.Errorf("unexpected ID: %s", j.ID)
	}
}

func TestGetJobNotFound(t *testing.T) {
	d := openTestDB(t)
	_, err := GetJob(d, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListJobs_NoFilter(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "id-1", "cmd1", 0)
	_ = InsertJob(d, "id-2", "cmd2", 0)
	jobs, err := ListJobs(d, "", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("len = %d, want 2", len(jobs))
	}
}

func TestListJobs_StatusFilter(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "id-1", "cmd1", 0)
	_ = InsertJob(d, "id-2", "cmd2", 0)
	_ = FinishJob(d, "id-1", 0, "", "")

	jobs, err := ListJobs(d, "pending", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "id-2" {
		t.Errorf("unexpected jobs: %+v", jobs)
	}
}

func TestListJobs_Limit(t *testing.T) {
	d := openTestDB(t)
	for i := range 5 {
		_ = InsertJob(d, string(rune('a'+i)), "cmd", 0)
	}
	jobs, err := ListJobs(d, "", 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("len = %d, want 2", len(jobs))
	}
}

func TestCancelJob_Pending(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "job-1", "sleep 10", 0)
	if err := CancelJob(d, "job-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	j, _ := GetJob(d, "job-1")
	if j.Status != StatusCancelled {
		t.Errorf("status = %q, want cancelled", j.Status)
	}
}

func TestCancelJob_Running(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "job-1", "sleep 10", 0)
	_, _ = ClaimPending(d, 1)
	if err := CancelJob(d, "job-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	j, _ := GetJob(d, "job-1")
	if j.Status != StatusCancelled {
		t.Errorf("status = %q, want cancelled", j.Status)
	}
}

func TestCancelJob_Done(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "job-1", "echo hi", 0)
	_ = FinishJob(d, "job-1", 0, "", "")
	err := CancelJob(d, "job-1")
	if err == nil {
		t.Fatal("expected error cancelling done job, got nil")
	}
}

func TestCancelJob_PrefixMatch(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "abcd1234-0000-0000-0000-000000000000", "sleep 10", 0)
	if err := CancelJob(d, "abcd1234"); err != nil {
		t.Fatalf("cancel by prefix: %v", err)
	}
	j, _ := GetJob(d, "abcd1234")
	if j.Status != StatusCancelled {
		t.Errorf("status = %q, want cancelled", j.Status)
	}
}

func TestClaimPending_PriorityOrder(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "low", "cmd", 0)
	_ = InsertJob(d, "high", "cmd", 10)
	jobs, err := ClaimPending(d, 1)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "high" {
		t.Errorf("expected high-priority job, got %+v", jobs)
	}
}

func TestClaimPending_FIFO(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "first", "cmd", 0)
	_ = InsertJob(d, "second", "cmd", 0)
	jobs, err := ClaimPending(d, 1)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "first" {
		t.Errorf("expected first job (FIFO), got %+v", jobs)
	}
}

func TestClaimPending_Limit(t *testing.T) {
	d := openTestDB(t)
	for i := range 5 {
		_ = InsertJob(d, string(rune('a'+i)), "cmd", 0)
	}
	jobs, err := ClaimPending(d, 3)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(jobs) != 3 {
		t.Errorf("len = %d, want 3", len(jobs))
	}
}

func TestFinishJob_Done(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "job-1", "echo hi", 0)
	if err := FinishJob(d, "job-1", 0, "out", ""); err != nil {
		t.Fatalf("finish: %v", err)
	}
	j, _ := GetJob(d, "job-1")
	if j.Status != StatusDone {
		t.Errorf("status = %q, want done", j.Status)
	}
	if j.Stdout != "out" {
		t.Errorf("stdout = %q, want %q", j.Stdout, "out")
	}
}

func TestFinishJob_Failed(t *testing.T) {
	d := openTestDB(t)
	_ = InsertJob(d, "job-1", "false", 0)
	if err := FinishJob(d, "job-1", 1, "", "err msg"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	j, _ := GetJob(d, "job-1")
	if j.Status != StatusFailed {
		t.Errorf("status = %q, want failed", j.Status)
	}
	if j.Stderr != "err msg" {
		t.Errorf("stderr = %q, want %q", j.Stderr, "err msg")
	}
}

func insertFinishedAt(t *testing.T, d *sql.DB, id, status string, finishedAt time.Time) {
	t.Helper()
	_ = InsertJob(d, id, "cmd", 0)
	_, err := d.Exec(
		`UPDATE jobs SET status=?, finished_at=? WHERE id=?`,
		status, finishedAt.UTC().Format("2006-01-02T15:04:05Z"), id,
	)
	if err != nil {
		t.Fatalf("insertFinishedAt: %v", err)
	}
}

func TestDeleteExpired_Done(t *testing.T) {
	d := openTestDB(t)
	insertFinishedAt(t, d, "old", "done", time.Now().Add(-15*24*time.Hour))
	insertFinishedAt(t, d, "new", "done", time.Now().Add(-1*24*time.Hour))

	policy := RetentionPolicy{Done: 14 * 24 * time.Hour}
	if err := DeleteExpired(d, policy); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := GetJob(d, "old"); err == nil {
		t.Error("expected old done job to be deleted")
	}
	if _, err := GetJob(d, "new"); err != nil {
		t.Errorf("expected new done job to remain: %v", err)
	}
}

func TestDeleteExpired_Failed(t *testing.T) {
	d := openTestDB(t)
	insertFinishedAt(t, d, "old", "failed", time.Now().Add(-31*24*time.Hour))
	insertFinishedAt(t, d, "new", "failed", time.Now().Add(-1*24*time.Hour))

	policy := RetentionPolicy{Failed: 30 * 24 * time.Hour}
	if err := DeleteExpired(d, policy); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := GetJob(d, "old"); err == nil {
		t.Error("expected old failed job to be deleted")
	}
	if _, err := GetJob(d, "new"); err != nil {
		t.Errorf("expected new failed job to remain: %v", err)
	}
}

func TestDeleteExpired_Cancelled(t *testing.T) {
	d := openTestDB(t)
	insertFinishedAt(t, d, "old", "cancelled", time.Now().Add(-8*24*time.Hour))
	insertFinishedAt(t, d, "new", "cancelled", time.Now().Add(-1*24*time.Hour))

	policy := RetentionPolicy{Cancelled: 7 * 24 * time.Hour}
	if err := DeleteExpired(d, policy); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := GetJob(d, "old"); err == nil {
		t.Error("expected old cancelled job to be deleted")
	}
	if _, err := GetJob(d, "new"); err != nil {
		t.Errorf("expected new cancelled job to remain: %v", err)
	}
}

func TestDeleteExpired_ZeroDuration(t *testing.T) {
	d := openTestDB(t)
	insertFinishedAt(t, d, "job-1", "done", time.Now().Add(-100*24*time.Hour))

	policy := RetentionPolicy{Done: 0}
	if err := DeleteExpired(d, policy); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := GetJob(d, "job-1"); err != nil {
		t.Error("expected job to remain when duration=0 (keep forever)")
	}
}

func TestDeleteExpired_NotYetExpired(t *testing.T) {
	d := openTestDB(t)
	insertFinishedAt(t, d, "job-1", "done", time.Now().Add(-3*24*time.Hour))

	policy := RetentionPolicy{Done: 14 * 24 * time.Hour}
	if err := DeleteExpired(d, policy); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := GetJob(d, "job-1"); err != nil {
		t.Error("expected unexpired job to remain")
	}
}
