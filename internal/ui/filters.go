package ui

import (
	"charm.land/huh/v2"

	"github.com/desenyon/ModelTUI/internal/catalog"
)

type capFilter string

const (
	capReasoning   capFilter = "reasoning"
	capTools       capFilter = "tools"
	capAttach      capFilter = "attachments"
	capOpenWeights capFilter = "open-weights"
	capStructured  capFilter = "structured-output"
	capMultimodal  capFilter = "multimodal"
	capFree        capFilter = "free"
)

func newFilterForm(selected *[]string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Capability filters").
				Description("Narrow the current tab. Leave empty to show everything.").
				Options(
					huh.NewOption("Reasoning", string(capReasoning)),
					huh.NewOption("Tool calling", string(capTools)),
					huh.NewOption("Attachments", string(capAttach)),
					huh.NewOption("Open weights", string(capOpenWeights)),
					huh.NewOption("Structured output", string(capStructured)),
					huh.NewOption("Multimodal input", string(capMultimodal)),
					huh.NewOption("Free / $0 pricing", string(capFree)),
				).
				Value(selected),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeCharm))
}

func matchCapsModel(m catalog.CanonicalModel, selected []string) bool {
	return catalog.MatchModelCapabilities(m, selected)
}

func matchCapsOffering(o catalog.Offering, selected []string) bool {
	return catalog.MatchOfferingCapabilities(o, selected)
}
