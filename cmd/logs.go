package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryomappi/kaiten/internal/db"
)

var logsCmd = &cobra.Command{
	Use:   "logs <job-id>",
	Short: "Show stdout/stderr of a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer database.Close()

		j, err := db.GetJob(database, args[0])
		if err != nil {
			return err
		}

		if j.Stdout != "" {
			fmt.Print(j.Stdout)
		}
		if j.Stderr != "" {
			fmt.Fprint(cmd.ErrOrStderr(), j.Stderr)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
