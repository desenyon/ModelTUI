package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/desenyon/ModelTUI/internal/catalog"
	"github.com/desenyon/ModelTUI/internal/format"
	"github.com/spf13/cobra"
)

func newListCommand(client *catalog.Client) *cobra.Command {
	var q catalog.Query
	output := "table"
	cmd := &cobra.Command{
		Use: "list", Short: "Search and compare provider offerings without starting the TUI", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if output != "table" && output != "json" {
				return fmt.Errorf("format must be table or json")
			}
			if err := q.Validate(); err != nil {
				return err
			}
			cat, source, err := client.LoadCatalog(cmd.Context())
			if err != nil {
				return err
			}
			offerings, err := catalog.BuildIndex(cat, source).QueryOfferings(q)
			if err != nil {
				return err
			}
			if output == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(struct {
					Source    string             `json:"source"`
					Count     int                `json:"count"`
					Offerings []catalog.Offering `json:"offerings"`
				}{source, len(offerings), offerings})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Source: %s · %d offerings\n", source, len(offerings))
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "PROVIDER\tMODEL\tCONTEXT\tINPUT / 1M\tOUTPUT / 1M")
			for _, o := range offerings {
				input, output := "—", "—"
				if o.Model.Cost != nil {
					input, output = format.Money(o.Model.Cost.Input), format.Money(o.Model.Cost.Output)
				}
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", o.ProviderID, o.Model.ID, o.Model.Limit.Context, input, output)
			}
			return w.Flush()
		},
	}
	f := cmd.Flags()
	f.StringVar(&q.Search, "query", "", "Case-insensitive substring across model and provider metadata")
	f.StringVar(&q.Provider, "provider", "", "Exact provider ID")
	f.StringSliceVar(&q.Capabilities, "capability", nil, "Required capabilities: reasoning,tools,attachments,open-weights,structured-output,multimodal,free")
	f.IntVar(&q.MinContext, "min-context", 0, "Minimum context window in tokens")
	f.StringVar(&q.Sort, "sort", "name", "Order: name, context (descending), input-price or output-price (ascending)")
	f.IntVar(&q.Limit, "limit", 0, "Maximum result count (0 means all)")
	f.StringVar(&output, "format", "table", "Output format: table or json")
	return cmd
}
