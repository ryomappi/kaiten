package worker

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ryomappi/kaiten/internal/db"
)

type Worker struct {
	DB       *sql.DB
	Workers  int
	PollFreq time.Duration
}

func (w *Worker) Run(ctx context.Context) {
	sem := make(chan struct{}, w.Workers)
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-time.After(w.PollFreq):
		}

		free := w.Workers - len(sem)
		if free <= 0 {
			continue
		}

		jobs, err := db.ClaimPending(w.DB, free)
		if err != nil {
			log.Printf("poll error: %v", err)
			continue
		}

		for _, j := range jobs {
			sem <- struct{}{}
			wg.Add(1)
			go func(id, command string) {
				defer func() { <-sem; wg.Done() }()
				w.execute(ctx, id, command)
			}(j.ID, j.Command)
		}
	}
}

func (w *Worker) execute(ctx context.Context, id, command string) {
	log.Printf("starting job %s: %s", id, command)

	parts := strings.Fields(command)
	if len(parts) == 0 {
		db.FinishJob(w.DB, id, 1, "", "empty command")
		return
	}

	// Check if cancelled before executing
	j, err := db.GetJob(w.DB, id)
	if err == nil && j.Status == db.StatusCancelled {
		return
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// If context cancelled, mark as cancelled
	if ctx.Err() != nil {
		db.MarkCancelledRunning(w.DB, id)
		log.Printf("job %s cancelled", id)
		return
	}

	if err := db.FinishJob(w.DB, id, exitCode, stdout.String(), stderr.String()); err != nil {
		log.Printf("finish job %s: %v", id, err)
	}
	log.Printf("finished job %s exit=%d", id, exitCode)
}
