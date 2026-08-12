package ui

import (
	"strings"

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

func hasCap(selected []string, want capFilter) bool {
	for _, s := range selected {
		if s == string(want) {
			return true
		}
	}
	return false
}

func matchCapsModel(m catalog.CanonicalModel, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	if hasCap(selected, capReasoning) && !m.Reasoning {
		return false
	}
	if hasCap(selected, capTools) && !m.ToolCall {
		return false
	}
	if hasCap(selected, capAttach) && !m.Attachment {
		return false
	}
	if hasCap(selected, capOpenWeights) && !m.OpenWeights {
		return false
	}
	if hasCap(selected, capStructured) && (m.StructuredOutput == nil || !*m.StructuredOutput) {
		return false
	}
	if hasCap(selected, capMultimodal) && !isMultimodal(m.Modalities) {
		return false
	}
	if hasCap(selected, capFree) {
		return false // canonical models have no pricing
	}
	return true
}

func matchCapsOffering(o catalog.Offering, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	m := o.Model
	if hasCap(selected, capReasoning) && !m.Reasoning {
		return false
	}
	if hasCap(selected, capTools) && !m.ToolCall {
		return false
	}
	if hasCap(selected, capAttach) && !m.Attachment {
		return false
	}
	if hasCap(selected, capOpenWeights) && !m.OpenWeights {
		return false
	}
	if hasCap(selected, capStructured) && (m.StructuredOutput == nil || !*m.StructuredOutput) {
		return false
	}
	if hasCap(selected, capMultimodal) && !isMultimodal(m.Modalities) {
		return false
	}
	if hasCap(selected, capFree) {
		if m.Cost == nil || m.Cost.Input != 0 || m.Cost.Output != 0 {
			return false
		}
	}
	return true
}

func isMultimodal(m *catalog.Modalities) bool {
	if m == nil {
		return false
	}
	for _, in := range m.Input {
		switch strings.ToLower(in) {
		case "image", "audio", "video", "pdf":
			return true
		}
	}
	return false
}
