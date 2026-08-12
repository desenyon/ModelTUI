package catalog

import (
	"sort"
	"strings"
)

// Index is a query-friendly view over a Catalog.
type Index struct {
	Catalog   *Catalog
	Source    string
	Models    []CanonicalModel
	Providers []Provider
	Offerings []Offering
	Labs      []Lab
}

// BuildIndex flattens and sorts catalog data for browsing.
func BuildIndex(cat *Catalog, source string) *Index {
	idx := &Index{Catalog: cat, Source: source}

	for _, m := range cat.Models {
		idx.Models = append(idx.Models, m)
	}
	sort.Slice(idx.Models, func(i, j int) bool {
		return strings.ToLower(idx.Models[i].Name) < strings.ToLower(idx.Models[j].Name)
	})

	for _, p := range cat.Providers {
		idx.Providers = append(idx.Providers, p)
	}
	sort.Slice(idx.Providers, func(i, j int) bool {
		return strings.ToLower(idx.Providers[i].Name) < strings.ToLower(idx.Providers[j].Name)
	})

	for _, p := range idx.Providers {
		modelIDs := make([]string, 0, len(p.Models))
		for id := range p.Models {
			modelIDs = append(modelIDs, id)
		}
		sort.Strings(modelIDs)
		for _, id := range modelIDs {
			m := p.Models[id]
			if m.ID == "" {
				m.ID = id
			}
			idx.Offerings = append(idx.Offerings, Offering{
				ProviderID:   p.ID,
				ProviderName: p.Name,
				Model:        m,
			})
		}
	}

	labMap := map[string][]CanonicalModel{}
	for _, m := range idx.Models {
		lab := LabID(m.ID)
		labMap[lab] = append(labMap[lab], m)
	}
	for id, models := range labMap {
		sort.Slice(models, func(i, j int) bool {
			return strings.ToLower(models[i].Name) < strings.ToLower(models[j].Name)
		})
		idx.Labs = append(idx.Labs, Lab{
			ID:     id,
			Name:   prettyLabName(id),
			Models: models,
		})
	}
	sort.Slice(idx.Labs, func(i, j int) bool {
		return strings.ToLower(idx.Labs[i].Name) < strings.ToLower(idx.Labs[j].Name)
	})

	return idx
}

// LabID extracts the lab/org prefix from a canonical model id.
func LabID(canonicalID string) string {
	if i := strings.IndexByte(canonicalID, '/'); i > 0 {
		return canonicalID[:i]
	}
	return canonicalID
}

func prettyLabName(id string) string {
	switch id {
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	case "google":
		return "Google"
	case "meta":
		return "Meta"
	case "xai":
		return "xAI"
	case "mistral":
		return "Mistral"
	case "cohere":
		return "Cohere"
	case "deepseek":
		return "DeepSeek"
	case "alibaba":
		return "Alibaba"
	case "nvidia":
		return "NVIDIA"
	case "zhipuai":
		return "Zhipu AI"
	case "moonshotai":
		return "Moonshot AI"
	case "minimax":
		return "MiniMax"
	case "xiaomi":
		return "Xiaomi"
	default:
		if id == "" {
			return "Unknown"
		}
		return strings.ToUpper(id[:1]) + id[1:]
	}
}

// LogoURL returns the models.dev logo URL for a provider or lab.
func LogoURL(kind, id string) string {
	switch kind {
	case "lab":
		return "https://models.dev/logos/labs/" + id + ".svg"
	default:
		return "https://models.dev/logos/" + id + ".svg"
	}
}

// OfferingsForCanonical returns provider offerings that match a canonical model.
func (idx *Index) OfferingsForCanonical(canonicalID string) []Offering {
	var out []Offering
	suffix := canonicalID
	if i := strings.IndexByte(canonicalID, '/'); i >= 0 {
		suffix = canonicalID[i+1:]
	}
	for _, o := range idx.Offerings {
		id := o.Model.ID
		if id == canonicalID || id == suffix || strings.HasSuffix(id, "/"+suffix) {
			out = append(out, o)
			continue
		}
		// Loose match on trailing segment.
		if j := strings.LastIndexByte(id, '/'); j >= 0 {
			if id[j+1:] == suffix {
				out = append(out, o)
			}
		}
	}
	return out
}
