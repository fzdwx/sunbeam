package cmd

import (
	"github.com/pomdtr/sunbeam/internal"
	"github.com/spf13/cobra"
)

func NewPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push <page>",
		Short: "Read page from file, and push it's content",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Draw(internal.NewFileGenerator(args[0]))
		},
	}

	return cmd
}