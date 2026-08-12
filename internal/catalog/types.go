package catalog

import "encoding/json"

// Catalog is the combined models.dev catalog payload.
type Catalog struct {
	Models    map[string]CanonicalModel `json:"models"`
	Providers map[string]Provider       `json:"providers"`
}

// Provider is a serving provider and its model offerings.
type Provider struct {
	ID     string                  `json:"id"`
	Name   string                  `json:"name"`
	API    string                  `json:"api,omitempty"`
	NPM    string                  `json:"npm,omitempty"`
	Doc    string                  `json:"doc,omitempty"`
	Env    []string                `json:"env"`
	Models map[string]OfferingModel `json:"models"`
}

// CanonicalModel is provider-agnostic model metadata from models.json.
type CanonicalModel struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Description      string         `json:"description,omitempty"`
	Family           string         `json:"family,omitempty"`
	Attachment       bool           `json:"attachment"`
	Reasoning        bool           `json:"reasoning"`
	ToolCall         bool           `json:"tool_call"`
	StructuredOutput *bool          `json:"structured_output,omitempty"`
	Temperature      bool           `json:"temperature"`
	Knowledge        string         `json:"knowledge,omitempty"`
	ReleaseDate      string         `json:"release_date,omitempty"`
	LastUpdated      string         `json:"last_updated,omitempty"`
	Modalities       *Modalities    `json:"modalities,omitempty"`
	OpenWeights      bool           `json:"open_weights"`
	Limit            Limit          `json:"limit"`
	License          string         `json:"license,omitempty"`
	Weights          []WeightLink   `json:"weights,omitempty"`
	Links            []WeightLink   `json:"links,omitempty"`
	Benchmarks       []Benchmark    `json:"benchmarks,omitempty"`
}

// OfferingModel is a provider-specific model offering from api.json.
type OfferingModel struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Description      string             `json:"description,omitempty"`
	Family           string             `json:"family,omitempty"`
	Attachment       bool               `json:"attachment"`
	Reasoning        bool               `json:"reasoning"`
	ReasoningOptions []ReasoningOption  `json:"reasoning_options,omitempty"`
	ToolCall         bool               `json:"tool_call"`
	StructuredOutput *bool              `json:"structured_output,omitempty"`
	Temperature      bool               `json:"temperature"`
	Interleaved      json.RawMessage    `json:"interleaved,omitempty"`
	Knowledge        string             `json:"knowledge,omitempty"`
	ReleaseDate      string             `json:"release_date,omitempty"`
	LastUpdated      string             `json:"last_updated,omitempty"`
	Modalities       *Modalities        `json:"modalities,omitempty"`
	OpenWeights      bool               `json:"open_weights"`
	Limit            Limit              `json:"limit"`
	Cost             *Cost              `json:"cost,omitempty"`
	Status           string             `json:"status,omitempty"`
	Experimental     json.RawMessage    `json:"experimental,omitempty"`
	Provider         *OfferingProvider  `json:"provider,omitempty"`
}

// OfferingProvider holds per-offering provider overrides.
type OfferingProvider struct {
	NPM   string          `json:"npm,omitempty"`
	API   string          `json:"api,omitempty"`
	Shape json.RawMessage `json:"shape,omitempty"`
}

// Modalities describes supported input/output media types.
type Modalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// Limit describes context window limits.
type Limit struct {
	Context int `json:"context"`
	Input   int `json:"input,omitempty"`
	Output  int `json:"output"`
}

// Cost is USD pricing per 1M tokens (and optional audio / tier fields).
type Cost struct {
	Input           float64    `json:"input"`
	Output          float64    `json:"output"`
	CacheRead       *float64   `json:"cache_read,omitempty"`
	CacheWrite      *float64   `json:"cache_write,omitempty"`
	Reasoning       *float64   `json:"reasoning,omitempty"`
	InputAudio      *float64   `json:"input_audio,omitempty"`
	OutputAudio     *float64   `json:"output_audio,omitempty"`
	Tiers           []CostTier `json:"tiers,omitempty"`
	ContextOver200k *CostFlat  `json:"context_over_200k,omitempty"`
}

// CostFlat is a flat cost block without nested tiers.
type CostFlat struct {
	Input      float64  `json:"input"`
	Output     float64  `json:"output"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
}

// CostTier is a contextual pricing tier.
type CostTier struct {
	Input      float64  `json:"input"`
	Output     float64  `json:"output"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
	Tier       struct {
		Type string `json:"type"`
		Size int    `json:"size"`
	} `json:"tier"`
}

// ReasoningOption describes controllable reasoning settings.
type ReasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
	Min    *float64 `json:"min,omitempty"`
	Max    *float64 `json:"max,omitempty"`
}

// WeightLink is a labeled URL (weights / docs / etc).
type WeightLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Benchmark is a published evaluation score.
type Benchmark struct {
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Metric  string  `json:"metric,omitempty"`
	Harness string  `json:"harness,omitempty"`
	Variant string  `json:"variant,omitempty"`
	Version string  `json:"version,omitempty"`
	Dataset string  `json:"dataset,omitempty"`
	Source  string  `json:"source,omitempty"`
	Date    string  `json:"date,omitempty"`
}

// Lab groups canonical models by author/org prefix.
type Lab struct {
	ID     string
	Name   string
	Models []CanonicalModel
}

// Offering is a flattened provider×model row for browsing.
type Offering struct {
	ProviderID   string
	ProviderName string
	Model        OfferingModel
}
