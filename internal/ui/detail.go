package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/desenyon/ModelTUI/internal/catalog"
	"github.com/desenyon/ModelTUI/internal/format"
)

func renderMarkdown(width int, md string) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(max(20, width)),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimRight(out, "\n")
}

func (m model) detailContent(width int) string {
	item, ok := m.selectedItem()
	if !ok {
		return m.theme.Desc.Render("Select an item to inspect every field from models.dev.")
	}
	switch item.kind {
	case kindModel:
		return m.renderCanonical(*item.model, width)
	case kindProvider:
		return m.renderProvider(*item.provider, width)
	case kindOffering:
		return m.renderOffering(*item.offering, width)
	case kindLab:
		return m.renderLab(*item.lab, width)
	default:
		return ""
	}
}

func (m model) kv(label, value string) string {
	return m.theme.Label.Render(label) + m.theme.Value.Render(value)
}

func (m model) section(title string) string {
	return m.theme.Section.Render("▸ " + title)
}

func (m model) boolRow(label string, v bool) string {
	glyph := m.theme.ChipOff.Render(format.BoolWord(v))
	if v {
		glyph = m.theme.ChipOn.Render(format.BoolWord(v))
	}
	return m.theme.Label.Render(label) + glyph
}

func (m model) renderCanonical(model catalog.CanonicalModel, width int) string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render(model.Name))
	b.WriteString("\n")
	b.WriteString(m.theme.Subtitle.Render(model.ID))
	b.WriteString("\n\n")

	if model.Description != "" {
		b.WriteString(renderMarkdown(width-2, model.Description))
		b.WriteString("\n")
	}

	b.WriteString(m.section("Identity"))
	b.WriteString("\n")
	b.WriteString(m.kv("Family", orDash(model.Family)) + "\n")
	b.WriteString(m.kv("Lab", catalog.LabID(model.ID)) + "\n")
	b.WriteString(m.kv("Lab logo", catalog.LogoURL("lab", catalog.LabID(model.ID))) + "\n")
	b.WriteString(m.kv("Knowledge", orDash(model.Knowledge)) + "\n")
	b.WriteString(m.kv("Released", orDash(model.ReleaseDate)) + "\n")
	b.WriteString(m.kv("Updated", orDash(model.LastUpdated)) + "\n")
	b.WriteString(m.kv("License", orDash(model.License)) + "\n")

	b.WriteString(m.section("Capabilities"))
	b.WriteString("\n")
	b.WriteString(m.boolRow("Reasoning", model.Reasoning) + "\n")
	b.WriteString(m.boolRow("Tool call", model.ToolCall) + "\n")
	b.WriteString(m.boolRow("Attachments", model.Attachment) + "\n")
	b.WriteString(m.boolRow("Temperature", model.Temperature) + "\n")
	b.WriteString(m.boolRow("Open weights", model.OpenWeights) + "\n")
	if model.StructuredOutput != nil {
		b.WriteString(m.boolRow("Structured out", *model.StructuredOutput) + "\n")
	} else {
		b.WriteString(m.kv("Structured out", "—") + "\n")
	}

	b.WriteString(m.section("Modalities"))
	b.WriteString("\n")
	b.WriteString(m.kv("I/O", format.Modalities(model.Modalities)) + "\n")

	b.WriteString(m.section("Limits"))
	b.WriteString("\n")
	b.WriteString(m.kv("Context", format.Tokens(model.Limit.Context)) + "\n")
	if model.Limit.Input > 0 {
		b.WriteString(m.kv("Input", format.Tokens(model.Limit.Input)) + "\n")
	}
	b.WriteString(m.kv("Output", format.Tokens(model.Limit.Output)) + "\n")
	b.WriteString(m.kv("Summary", format.LimitLine(model.Limit)) + "\n")

	if len(model.Weights) > 0 {
		b.WriteString(m.section("Weights"))
		b.WriteString("\n")
		for _, w := range model.Weights {
			b.WriteString(fmt.Sprintf("  • %s\n    %s\n", w.Label, lipgloss.NewStyle().Foreground(m.theme.AccentAlt).Render(w.URL)))
		}
	}
	if len(model.Links) > 0 {
		b.WriteString(m.section("Links"))
		b.WriteString("\n")
		for _, w := range model.Links {
			b.WriteString(fmt.Sprintf("  • %s\n    %s\n", w.Label, lipgloss.NewStyle().Foreground(m.theme.AccentAlt).Render(w.URL)))
		}
	}
	if len(model.Benchmarks) > 0 {
		b.WriteString(m.section(fmt.Sprintf("Benchmarks (%d)", len(model.Benchmarks))))
		b.WriteString("\n")
		b.WriteString(renderBenchmarkTable(model.Benchmarks, m.theme, width))
		b.WriteString("\n")
	}

	if m.index != nil {
		offers := m.index.OfferingsForCanonical(model.ID)
		b.WriteString(m.section(fmt.Sprintf("Providers serving this model (%d)", len(offers))))
		b.WriteString("\n")
		if len(offers) == 0 {
			b.WriteString(m.theme.Desc.Render("  No exact provider offerings matched.") + "\n")
		}
		for _, o := range offers {
			price := "n/a"
			if o.Model.Cost != nil {
				price = fmt.Sprintf("%s in / %s out", format.Money(o.Model.Cost.Input), format.Money(o.Model.Cost.Output))
			}
			status := o.Model.Status
			if status == "" {
				status = "stable"
			}
			b.WriteString(fmt.Sprintf("  • %s (%s) · %s · %s · ctx %s\n",
				o.ProviderName, o.ProviderID, o.Model.ID, price, format.Tokens(o.Model.Limit.Context)))
			_ = status
		}
	}

	return b.String()
}

func (m model) renderProvider(p catalog.Provider, width int) string {
	_ = width
	var b strings.Builder
	b.WriteString(m.theme.Title.Render(p.Name))
	b.WriteString("\n")
	b.WriteString(m.theme.Subtitle.Render(p.ID))
	b.WriteString("\n\n")

	b.WriteString(m.section("Provider"))
	b.WriteString("\n")
	b.WriteString(m.kv("API", orDash(p.API)) + "\n")
	b.WriteString(m.kv("NPM", orDash(p.NPM)) + "\n")
	b.WriteString(m.kv("Docs", orDash(p.Doc)) + "\n")
	b.WriteString(m.kv("Logo", catalog.LogoURL("provider", p.ID)) + "\n")
	b.WriteString(m.kv("Env vars", orDash(strings.Join(p.Env, ", "))) + "\n")
	b.WriteString(m.kv("Models", fmt.Sprintf("%d", len(p.Models))) + "\n")

	b.WriteString(m.section("Model offerings"))
	b.WriteString("\n")
	ids := make([]string, 0, len(p.Models))
	for id := range p.Models {
		ids = append(ids, id)
	}
	// Keep deterministic-ish order by name via offerings already sorted globally — local map dump:
	for _, id := range sortedStrings(ids) {
		om := p.Models[id]
		price := "n/a"
		if om.Cost != nil {
			price = fmt.Sprintf("%s/%s", format.Money(om.Cost.Input), format.Money(om.Cost.Output))
		}
		b.WriteString(fmt.Sprintf("  • %s\n    id %s · ctx %s · %s",
			om.Name, om.ID, format.Tokens(om.Limit.Context), price))
		flags := capabilityShort(om.Reasoning, om.ToolCall, om.Attachment, om.OpenWeights, om.StructuredOutput)
		if flags != "" {
			b.WriteString(" · " + flags)
		}
		if om.Status != "" {
			b.WriteString(" · " + om.Status)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) renderOffering(o catalog.Offering, width int) string {
	model := o.Model
	var b strings.Builder
	b.WriteString(m.theme.Title.Render(model.Name))
	b.WriteString("\n")
	b.WriteString(m.theme.Subtitle.Render(fmt.Sprintf("%s / %s", o.ProviderName, model.ID)))
	b.WriteString("\n\n")

	if model.Description != "" {
		b.WriteString(renderMarkdown(width-2, model.Description))
		b.WriteString("\n")
	}

	b.WriteString(m.section("Serving provider"))
	b.WriteString("\n")
	b.WriteString(m.kv("Provider", o.ProviderName) + "\n")
	b.WriteString(m.kv("Provider ID", o.ProviderID) + "\n")
	b.WriteString(m.kv("Provider logo", catalog.LogoURL("provider", o.ProviderID)) + "\n")
	if prov, ok := m.index.Catalog.Providers[o.ProviderID]; ok {
		b.WriteString(m.kv("API", orDash(prov.API)) + "\n")
		b.WriteString(m.kv("NPM", orDash(prov.NPM)) + "\n")
		b.WriteString(m.kv("Docs", orDash(prov.Doc)) + "\n")
		b.WriteString(m.kv("Env", orDash(strings.Join(prov.Env, ", "))) + "\n")
	}

	b.WriteString(m.section("Identity"))
	b.WriteString("\n")
	b.WriteString(m.kv("Model ID", model.ID) + "\n")
	b.WriteString(m.kv("Family", orDash(model.Family)) + "\n")
	b.WriteString(m.kv("Status", orDash(orDefault(model.Status, "stable"))) + "\n")
	b.WriteString(m.kv("Knowledge", orDash(model.Knowledge)) + "\n")
	b.WriteString(m.kv("Released", orDash(model.ReleaseDate)) + "\n")
	b.WriteString(m.kv("Updated", orDash(model.LastUpdated)) + "\n")

	b.WriteString(m.section("Capabilities"))
	b.WriteString("\n")
	b.WriteString(m.boolRow("Reasoning", model.Reasoning) + "\n")
	b.WriteString(m.boolRow("Tool call", model.ToolCall) + "\n")
	b.WriteString(m.boolRow("Attachments", model.Attachment) + "\n")
	b.WriteString(m.boolRow("Temperature", model.Temperature) + "\n")
	b.WriteString(m.boolRow("Open weights", model.OpenWeights) + "\n")
	if model.StructuredOutput != nil {
		b.WriteString(m.boolRow("Structured out", *model.StructuredOutput) + "\n")
	} else {
		b.WriteString(m.kv("Structured out", "—") + "\n")
	}

	if len(model.ReasoningOptions) > 0 {
		b.WriteString(m.section("Reasoning options"))
		b.WriteString("\n")
		for _, ro := range model.ReasoningOptions {
			line := fmt.Sprintf("  • type=%s", ro.Type)
			if len(ro.Values) > 0 {
				line += " values=[" + strings.Join(ro.Values, ", ") + "]"
			}
			if ro.Min != nil || ro.Max != nil {
				line += fmt.Sprintf(" range=[%v, %v]", ro.Min, ro.Max)
			}
			b.WriteString(line + "\n")
		}
	}

	if len(model.Interleaved) > 0 {
		b.WriteString(m.section("Interleaved reasoning"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(m.theme.Muted).Render(format.PrettyJSON(model.Interleaved)))
		b.WriteString("\n")
	}

	b.WriteString(m.section("Modalities"))
	b.WriteString("\n")
	b.WriteString(m.kv("I/O", format.Modalities(model.Modalities)) + "\n")

	b.WriteString(m.section("Limits"))
	b.WriteString("\n")
	b.WriteString(m.kv("Context", format.Tokens(model.Limit.Context)) + "\n")
	if model.Limit.Input > 0 {
		b.WriteString(m.kv("Input", format.Tokens(model.Limit.Input)) + "\n")
	}
	b.WriteString(m.kv("Output", format.Tokens(model.Limit.Output)) + "\n")

	b.WriteString(m.section("Pricing (USD / 1M tokens)"))
	b.WriteString("\n")
	b.WriteString(format.CostBlock(model.Cost) + "\n")

	if model.Provider != nil {
		b.WriteString(m.section("Offering provider overrides"))
		b.WriteString("\n")
		b.WriteString(m.kv("NPM", orDash(model.Provider.NPM)) + "\n")
		b.WriteString(m.kv("API", orDash(model.Provider.API)) + "\n")
		if len(model.Provider.Shape) > 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(m.theme.Muted).Render(format.PrettyJSON(model.Provider.Shape)) + "\n")
		}
	}

	if len(model.Experimental) > 0 {
		b.WriteString(m.section("Experimental"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(m.theme.Muted).Render(format.PrettyJSON(model.Experimental)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) renderLab(lab catalog.Lab, width int) string {
	_ = width
	var b strings.Builder
	b.WriteString(m.theme.Title.Render(lab.Name))
	b.WriteString("\n")
	b.WriteString(m.theme.Subtitle.Render(lab.ID))
	b.WriteString("\n\n")
	b.WriteString(m.section("Lab"))
	b.WriteString("\n")
	b.WriteString(m.kv("Logo", catalog.LogoURL("lab", lab.ID)) + "\n")
	b.WriteString(m.kv("Canonical models", fmt.Sprintf("%d", len(lab.Models))) + "\n")
	b.WriteString(m.section("Models"))
	b.WriteString("\n")
	for _, model := range lab.Models {
		b.WriteString(fmt.Sprintf("  • %s (%s) · ctx %s · %s\n",
			model.Name,
			model.ID,
			format.Tokens(model.Limit.Context),
			capabilityShort(model.Reasoning, model.ToolCall, model.Attachment, model.OpenWeights, model.StructuredOutput),
		))
	}
	return b.String()
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func renderBenchmarkTable(bms []catalog.Benchmark, theme Theme, width int) string {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(theme.Border)).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Foreground(theme.AccentAlt)
			}
			if col == 1 {
				return lipgloss.NewStyle().Foreground(theme.Good)
			}
			return lipgloss.NewStyle().Foreground(theme.Fg)
		}).
		Headers("Benchmark", "Score", "Metric", "Harness", "Variant")

	limit := len(bms)
	if limit > 40 {
		limit = 40
	}
	for i := 0; i < limit; i++ {
		bm := bms[i]
		t.Row(
			truncate(bm.Name, max(12, width/3)),
			fmt.Sprintf("%.4g", bm.Score),
			orDash(bm.Metric),
			orDash(bm.Harness),
			orDash(bm.Variant),
		)
	}
	out := t.String()
	if len(bms) > limit {
		out += "\n" + lipgloss.NewStyle().Foreground(theme.Muted).Render(
			fmt.Sprintf("… and %d more benchmarks", len(bms)-limit),
		)
	}
	// Sources below the table for completeness.
	var sources strings.Builder
	seen := map[string]struct{}{}
	for _, bm := range bms[:limit] {
		if bm.Source == "" {
			continue
		}
		if _, ok := seen[bm.Source]; ok {
			continue
		}
		seen[bm.Source] = struct{}{}
		sources.WriteString("\n  " + lipgloss.NewStyle().Foreground(theme.AccentAlt).Render(bm.Source))
	}
	if sources.Len() > 0 {
		out += "\n" + lipgloss.NewStyle().Foreground(theme.Muted).Render("Sources:") + sources.String()
	}
	return out
}
