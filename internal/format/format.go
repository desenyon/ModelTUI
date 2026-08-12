package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/desenyon/ModelTUI/internal/catalog"
)

// Tokens formats a token count for display.
func Tokens(n int) string {
	if n <= 0 {
		return "—"
	}
	switch {
	case n >= 1_000_000:
		v := float64(n) / 1_000_000
		if math.Mod(v, 1) == 0 {
			return fmt.Sprintf("%.0fM", v)
		}
		return fmt.Sprintf("%.2fM", v)
	case n >= 1_000:
		v := float64(n) / 1_000
		if math.Mod(v, 1) == 0 {
			return fmt.Sprintf("%.0fK", v)
		}
		return fmt.Sprintf("%.1fK", v)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// Money formats USD per 1M tokens.
func Money(v float64) string {
	if v == 0 {
		return "$0"
	}
	if v < 0.01 {
		return fmt.Sprintf("$%.4f", v)
	}
	if v < 1 {
		return fmt.Sprintf("$%.3f", v)
	}
	if math.Mod(v, 1) == 0 {
		return fmt.Sprintf("$%.0f", v)
	}
	return fmt.Sprintf("$%.2f", v)
}

// MoneyPtr formats an optional float cost.
func MoneyPtr(v *float64) string {
	if v == nil {
		return "—"
	}
	return Money(*v)
}

// Bool returns a glyph for boolean capability.
func Bool(v bool) string {
	if v {
		return "●"
	}
	return "○"
}

// BoolWord returns yes/no.
func BoolWord(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// Join joins strings with a middle dot.
func Join(parts ...string) string {
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// Modalities renders input→output modalities.
func Modalities(m *catalog.Modalities) string {
	if m == nil {
		return "—"
	}
	in := "—"
	out := "—"
	if len(m.Input) > 0 {
		in = strings.Join(m.Input, ", ")
	}
	if len(m.Output) > 0 {
		out = strings.Join(m.Output, ", ")
	}
	return in + " → " + out
}

// PrettyJSON indents raw JSON for detail panes.
func PrettyJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}

// CostBlock renders a full cost section.
func CostBlock(c *catalog.Cost) string {
	if c == nil {
		return "Pricing: unavailable"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Input %s / 1M · Output %s / 1M", Money(c.Input), Money(c.Output)))
	if c.CacheRead != nil || c.CacheWrite != nil {
		b.WriteString(fmt.Sprintf("\nCache read %s · Cache write %s", MoneyPtr(c.CacheRead), MoneyPtr(c.CacheWrite)))
	}
	if c.Reasoning != nil {
		b.WriteString(fmt.Sprintf("\nReasoning %s / 1M", MoneyPtr(c.Reasoning)))
	}
	if c.InputAudio != nil || c.OutputAudio != nil {
		b.WriteString(fmt.Sprintf("\nAudio in %s · Audio out %s", MoneyPtr(c.InputAudio), MoneyPtr(c.OutputAudio)))
	}
	if c.ContextOver200k != nil {
		b.WriteString(fmt.Sprintf(
			"\nOver 200K context: in %s · out %s · cache read %s · cache write %s",
			Money(c.ContextOver200k.Input),
			Money(c.ContextOver200k.Output),
			MoneyPtr(c.ContextOver200k.CacheRead),
			MoneyPtr(c.ContextOver200k.CacheWrite),
		))
	}
	for _, t := range c.Tiers {
		b.WriteString(fmt.Sprintf(
			"\nTier %s≥%s: in %s · out %s · cache read %s · cache write %s",
			t.Tier.Type,
			Tokens(t.Tier.Size),
			Money(t.Input),
			Money(t.Output),
			MoneyPtr(t.CacheRead),
			MoneyPtr(t.CacheWrite),
		))
	}
	return b.String()
}

// LimitLine renders context/input/output limits.
func LimitLine(l catalog.Limit) string {
	parts := []string{fmt.Sprintf("context %s", Tokens(l.Context))}
	if l.Input > 0 {
		parts = append(parts, fmt.Sprintf("input %s", Tokens(l.Input)))
	}
	parts = append(parts, fmt.Sprintf("output %s", Tokens(l.Output)))
	return strings.Join(parts, " · ")
}
