package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ryomappi/kaiten/internal/db"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List jobs in the queue",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		limit, _ := cmd.Flags().GetInt("limit")

		database, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer database.Close()

		jobs, err := db.ListJobs(database, status, limit)
		if err != nil {
			return err
		}

		if len(jobs) == 0 {
			fmt.Println("no jobs found")
			return nil
		}

		fmt.Fprintf(os.Stdout, "%-8s  %-9s  %3s  %-19s  %s\n", "ID", "STATUS", "PRI", "CREATED", "COMMAND")
		for _, j := range jobs {
			command := j.Command
			if len(command) > 40 {
				command = command[:37] + "..."
			}
			fmt.Fprintf(os.Stdout, "%s  %s  %3d  %s  %s\n",
				j.ID[:8],
				colorStatus(string(j.Status)),
				j.Priority,
				j.CreatedAt.Format(time.DateTime),
				command,
			)
		}
		return nil
	},
}

func colorStatus(s string) string {
	padded := fmt.Sprintf("%-9s", s) // "cancelled" is the longest status with 9 chars
	switch s {
	case "running":
		return "\033[33m" + padded + "\033[0m"
	case "done":
		return "\033[32m" + padded + "\033[0m"
	case "failed":
		return "\033[31m" + padded + "\033[0m"
	case "cancelled":
		return "\033[90m" + padded + "\033[0m"
	}
	return padded
}

func init() {
	listCmd.Flags().StringP("status", "s", "", "filter by status: pending|running|done|failed|cancelled")
	listCmd.Flags().IntP("limit", "n", 50, "max number of jobs to show")
	rootCmd.AddCommand(listCmd)
}
