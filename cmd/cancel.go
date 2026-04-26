package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryomappi/kaiten/internal/db"
)

var cancelCmd = &cobra.Command{
	Use:   "cancel <job-id>",
	Short: "Cancel a pending or running job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer database.Close()

		if err := db.CancelJob(database, args[0]); err != nil {
			return err
		}
		fmt.Printf("cancelled %s\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cancelCmd)
}
