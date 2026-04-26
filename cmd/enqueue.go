package cmd

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/ryomappi/kaiten/internal/db"
)

var enqueueCmd = &cobra.Command{
	Use:   "enqueue -- <command> [args...]",
	Short: "Add a job to the queue",
	Example: `  kaiten enqueue -- echo hello
  kaiten enqueue --priority 10 -- python3 script.py arg1`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		priority, _ := cmd.Flags().GetInt("priority")
		command := strings.Join(args, " ")

		database, err := db.Open(dbPath)
		if err != nil {
			return err
		}
		defer database.Close()

		id := uuid.New().String()
		if err := db.InsertJob(database, id, command, priority); err != nil {
			return err
		}
		fmt.Println(id)
		return nil
	},
}

func init() {
	enqueueCmd.Flags().IntP("priority", "p", 0, "job priority (higher = runs first)")
	rootCmd.AddCommand(enqueueCmd)
}
