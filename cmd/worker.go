package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/ryomappi/kaiten/internal/db"
	"github.com/ryomappi/kaiten/internal/worker"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start the job worker daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		numWorkers, _ := cmd.Flags().GetInt("workers")
		pollStr, _ := cmd.Flags().GetString("poll")
		pollFreq, err := time.ParseDuration(pollStr)
		if err != nil {
			return err
		}

		retainDone, _ := cmd.Flags().GetInt("retain-done")
		retainFailed, _ := cmd.Flags().GetInt("retain-failed")
		retainCancelled, _ := cmd.Flags().GetInt("retain-cancelled")

		database, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer database.Close()

		log.Printf("worker started: db=%s workers=%d poll=%s retain(done=%dd failed=%dd cancelled=%dd)",
			dbPath, numWorkers, pollFreq, retainDone, retainFailed, retainCancelled)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		w := &worker.Worker{
			DB:       database,
			Workers:  numWorkers,
			PollFreq: pollFreq,
			Retention: db.RetentionPolicy{
				Done:      time.Duration(retainDone) * 24 * time.Hour,
				Failed:    time.Duration(retainFailed) * 24 * time.Hour,
				Cancelled: time.Duration(retainCancelled) * 24 * time.Hour,
			},
		}
		w.Run(ctx)
		log.Println("worker stopped")
		return nil
	},
}

func init() {
	workerCmd.Flags().IntP("workers", "w", 4, "number of parallel workers")
	workerCmd.Flags().String("poll", "1s", "polling interval (e.g. 500ms, 2s)")
	workerCmd.Flags().Int("retain-done", 14, "days to retain done jobs (0 = forever)")
	workerCmd.Flags().Int("retain-failed", 30, "days to retain failed jobs (0 = forever)")
	workerCmd.Flags().Int("retain-cancelled", 7, "days to retain cancelled jobs (0 = forever)")
	rootCmd.AddCommand(workerCmd)
}
