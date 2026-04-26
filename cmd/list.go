package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
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

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tPRI\tCREATED\tCOMMAND")
		for _, j := range jobs {
			cmd := j.Command
			if len(cmd) > 40 {
				cmd = cmd[:37] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				j.ID[:8],
				colorStatus(string(j.Status)),
				j.Priority,
				j.CreatedAt.Format(time.DateTime),
				cmd,
			)
		}
		return w.Flush()
	},
}

func colorStatus(s string) string {
	switch s {
	case "pending":
		return s
	case "running":
		return "\033[33m" + s + "\033[0m"
	case "done":
		return "\033[32m" + s + "\033[0m"
	case "failed":
		return "\033[31m" + s + "\033[0m"
	case "cancelled":
		return "\033[90m" + s + "\033[0m"
	}
	return s
}

func init() {
	listCmd.Flags().StringP("status", "s", "", "filter by status: pending|running|done|failed|cancelled")
	listCmd.Flags().IntP("limit", "n", 50, "max number of jobs to show")
	rootCmd.AddCommand(listCmd)
}
