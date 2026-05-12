package main

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	tw "github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"

	"github.com/laguilar-io/devbrowser/internal/state"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all running devbrowser sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := state.All()
		if err != nil {
			return err
		}
		if len(all) == 0 {
			fmt.Println("No active sessions.")
			return nil
		}

		table := tablewriter.NewTable(os.Stdout,
			tablewriter.WithHeader([]string{"Worktree", "Port", "Status", "Command", "Started"}),
			tablewriter.WithBorders(tw.Border{Left: tw.Off, Right: tw.Off, Top: tw.Off, Bottom: tw.Off}),
		)

		for name, entry := range all {
			status := color.GreenString("running")
			if !isAlive(entry.ServerPID) {
				status = color.YellowString("stale")
			}
			table.Append([]string{
				name,
				fmt.Sprintf("%d", entry.Port),
				status,
				entry.Command,
				humanAge(entry.StartedAt),
			})
		}
		table.Render()
		return nil
	},
}

func isAlive(pid int) bool { return isProcessAlive(pid) }

func humanAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
