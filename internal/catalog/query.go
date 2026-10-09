package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// Query filters provider offerings. Zero values select every offering by name.
type Query struct {
	Search       string
	Provider     string
	Capabilities []string
	MinContext   int
	Sort         string
	Limit        int
}

var capabilities = map[string]bool{"reasoning": true, "tools": true, "attachments": true, "open-weights": true, "structured-output": true, "multimodal": true, "free": true}

func (q Query) Validate() error {
	if q.MinContext < 0 || q.Limit < 0 {
		return fmt.Errorf("min-context and limit must be non-negative")
	}
	switch q.Sort {
	case "", "name", "context", "input-price", "output-price":
	default:
		return fmt.Errorf("unknown sort %q (use name, context, input-price or output-price)", q.Sort)
	}
	for _, cap := range q.Capabilities {
		if !capabilities[cap] {
			return fmt.Errorf("unknown capability %q", cap)
		}
	}
	return nil
}

func (idx *Index) QueryOfferings(q Query) ([]Offering, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}
	out := make([]Offering, 0)
	search := strings.ToLower(strings.TrimSpace(q.Search))
	for _, o := range idx.Offerings {
		m := o.Model
		if q.Provider != "" && o.ProviderID != q.Provider {
			continue
		}
		if m.Limit.Context < q.MinContext || !MatchOfferingCapabilities(o, q.Capabilities) {
			continue
		}
		haystack := strings.ToLower(strings.Join([]string{o.ProviderID, o.ProviderName, m.ID, m.Name, m.Family, m.Description, m.Status}, " "))
		if !strings.Contains(haystack, search) {
			continue
		}
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch q.Sort {
		case "context":
			if a.Model.Limit.Context != b.Model.Limit.Context {
				return a.Model.Limit.Context > b.Model.Limit.Context
			}
		case "input-price", "output-price":
			ac, bc := a.Model.Cost, b.Model.Cost
			if (ac == nil) != (bc == nil) {
				return ac != nil
			}
			if ac != nil && bc != nil {
				ap, bp := ac.Input, bc.Input
				if q.Sort == "output-price" {
					ap, bp = ac.Output, bc.Output
				}
				if ap != bp {
					return ap < bp
				}
			}
		}
		an, bn := strings.ToLower(a.Model.Name), strings.ToLower(b.Model.Name)
		if an != bn {
			return an < bn
		}
		if a.ProviderID != b.ProviderID {
			return a.ProviderID < b.ProviderID
		}
		return a.Model.ID < b.Model.ID
	})
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func MatchModelCapabilities(m CanonicalModel, selected []string) bool {
	return matchCapabilities(m.Reasoning, m.ToolCall, m.Attachment, m.OpenWeights, m.StructuredOutput, m.Modalities, nil, selected)
}
func MatchOfferingCapabilities(o Offering, selected []string) bool {
	m := o.Model
	return matchCapabilities(m.Reasoning, m.ToolCall, m.Attachment, m.OpenWeights, m.StructuredOutput, m.Modalities, m.Cost, selected)
}
func matchCapabilities(reasoning, tools, attachments, open bool, structured *bool, modalities *Modalities, cost *Cost, selected []string) bool {
	for _, cap := range selected {
		matched := false
		switch cap {
		case "reasoning":
			matched = reasoning
		case "tools":
			matched = tools
		case "attachments":
			matched = attachments
		case "open-weights":
			matched = open
		case "structured-output":
			matched = structured != nil && *structured
		case "free":
			matched = cost != nil && cost.Input == 0 && cost.Output == 0
		case "multimodal":
			if modalities != nil {
				for _, in := range modalities.Input {
					switch strings.ToLower(in) {
					case "image", "audio", "video", "pdf":
						matched = true
					}
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
