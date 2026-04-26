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

		database, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer database.Close()

		log.Printf("worker started: db=%s workers=%d poll=%s", dbPath, numWorkers, pollFreq)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		w := &worker.Worker{
			DB:       database,
			Workers:  numWorkers,
			PollFreq: pollFreq,
		}
		w.Run(ctx)
		log.Println("worker stopped")
		return nil
	},
}

func init() {
	workerCmd.Flags().IntP("workers", "w", 4, "number of parallel workers")
	workerCmd.Flags().String("poll", "1s", "polling interval (e.g. 500ms, 2s)")
	rootCmd.AddCommand(workerCmd)
}
