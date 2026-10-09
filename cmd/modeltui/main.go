package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/fang/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/desenyon/ModelTUI/internal/catalog"
	"github.com/desenyon/ModelTUI/internal/ui"
	"github.com/desenyon/ModelTUI/internal/update"
)

func newRootCommand() *cobra.Command {
	client := catalog.NewClient()
	timeout := 45 * time.Second
	root := &cobra.Command{
		Use: "modeltui", Short: "A glamorous TUI for the models.dev AI model catalog",
		Long:    "Browse models, providers, offerings and labs in an animated Charm TUI.\nUse modeltui list for searchable table or JSON output without a terminal.",
		Version: update.Version, Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if timeout <= 0 {
				return fmt.Errorf("timeout must be positive")
			}
			if client.CacheDir == "" {
				return fmt.Errorf("cache-dir must not be empty")
			}
			client.HTTPClient.Timeout = timeout
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := []tea.ProgramOption{tea.WithContext(cmd.Context())}
			if _, noColor := os.LookupEnv("NO_COLOR"); !noColor {
				opts = append(opts, tea.WithColorProfile(colorprofile.TrueColor))
			}
			_, err := tea.NewProgram(ui.NewWithClient(cmd.Context(), client), opts...).Run()
			return err
		},
	}
	flags := root.PersistentFlags()
	flags.BoolVar(&client.Offline, "offline", false, "Use disk cache or embedded snapshot without catalog network requests")
	flags.StringVar(&client.CacheDir, "cache-dir", client.CacheDir, "Directory for catalog and rate metadata")
	flags.DurationVar(&timeout, "timeout", 45*time.Second, "Catalog HTTP request timeout (for example 10s)")
	root.AddCommand(newListCommand(client))
	root.AddCommand(&cobra.Command{
		Use: "update", Short: "Install the latest ModelTUI release archive", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if client.Offline {
				return catalog.ErrOffline
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
			defer cancel()
			res, err := update.Check(ctx, "desenyon/ModelTUI")
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "current=%s latest=%s\n", res.Current, res.Latest)
			if res.UpToDate {
				fmt.Fprintln(cmd.OutOrStdout(), "Already up to date.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s…\n", res.AssetName)
			if err := update.Apply(ctx, res.AssetURL); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Updated successfully. Restart modeltui.")
			return nil
		},
	})
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), update.Version)
		return err
	}})
	return root
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := fang.Execute(ctx, newRootCommand(), fang.WithVersion(update.Version)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
