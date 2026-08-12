package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/fang/v2"
	"github.com/spf13/cobra"

	"github.com/desenyon/ModelTUI/internal/ui"
	"github.com/desenyon/ModelTUI/internal/update"
)

func main() {
	root := &cobra.Command{
		Use:   "modeltui",
		Short: "A glamorous TUI for the models.dev AI model catalog",
		Long: `ModelTUI is a Charm-powered terminal UI for models.dev.

Browse canonical models, providers, offerings, and labs with live pricing,
capabilities, modalities, benchmarks, and every field exposed by the API.

Press space to refresh the catalog. Requests are spaced and honor HTTP 429.`,
		Version: update.Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := tea.NewProgram(ui.New())
			_, err := p.Run()
			return err
		},
	}

	root.AddCommand(&cobra.Command{
		Use:   "update",
		Short: "Check for and install the latest ModelTUI release",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
			defer cancel()
			res, err := update.Check(ctx, "desenyon/ModelTUI")
			if err != nil {
				return err
			}
			fmt.Printf("current=%s latest=%s\n", res.Current, res.Latest)
			if res.UpToDate {
				fmt.Println("Already up to date.")
				return nil
			}
			if res.AssetURL == "" {
				return fmt.Errorf("latest release has no binary for this platform")
			}
			fmt.Printf("Downloading %s…\n", res.AssetName)
			if err := update.Apply(ctx, res.AssetURL); err != nil {
				return err
			}
			fmt.Println("Updated successfully. Restart modeltui.")
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(update.Version)
			return nil
		},
	})

	if err := fang.Execute(context.Background(), root, fang.WithVersion(update.Version)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
