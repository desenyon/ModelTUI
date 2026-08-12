package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Hex palette — CharmTone-inspired, explicit RGB so TrueColor always wins.
var (
	colBg         = lipgloss.Color("#1A1218")
	colFg         = lipgloss.Color("#FFF1C1")
	colMuted      = lipgloss.Color("#A89B9F")
	colSubtle     = lipgloss.Color("#6E6268")
	colAccent     = lipgloss.Color("#FF79D0")
	colAccentAlt  = lipgloss.Color("#7DD3FC")
	colGood       = lipgloss.Color("#86EFAC")
	colWarn       = lipgloss.Color("#FDE047")
	colBad        = lipgloss.Color("#FB7185")
	colInfo       = lipgloss.Color("#67E8F9")
	colBorder     = lipgloss.Color("#3F2A38")
	colBorderHot  = lipgloss.Color("#C259FF")
	colPanel      = lipgloss.Color("#221820")
	colSelectBg   = lipgloss.Color("#3A1830")
	colPepper     = lipgloss.Color("#1A1218")
)

// Theme holds styles for ModelTUI.
type Theme struct {
	Bg          color.Color
	Fg          color.Color
	Muted       color.Color
	Subtle      color.Color
	Accent      color.Color
	AccentAlt   color.Color
	Good        color.Color
	Warn        color.Color
	Bad         color.Color
	Info        color.Color
	Border      color.Color
	BorderFocus color.Color

	App        lipgloss.Style
	Header     lipgloss.Style
	Brand      lipgloss.Style
	Subtitle   lipgloss.Style
	TabActive  lipgloss.Style
	TabIdle    lipgloss.Style
	Panel      lipgloss.Style
	PanelFocus lipgloss.Style
	Section    lipgloss.Style
	Label      lipgloss.Style
	Value      lipgloss.Style
	ChipOn     lipgloss.Style
	ChipOff    lipgloss.Style
	Help       lipgloss.Style
	Status     lipgloss.Style
	Error      lipgloss.Style
	Title      lipgloss.Style
	Desc       lipgloss.Style
	Stat       lipgloss.Style
	Rule       lipgloss.Style
}

// NewTheme builds a vivid dark Charm-inspired palette.
func NewTheme() Theme {
	t := Theme{
		Bg:          colBg,
		Fg:          colFg,
		Muted:       colMuted,
		Subtle:      colSubtle,
		Accent:      colAccent,
		AccentAlt:   colAccentAlt,
		Good:        colGood,
		Warn:        colWarn,
		Bad:         colBad,
		Info:        colInfo,
		Border:      colBorder,
		BorderFocus: colBorderHot,
	}

	t.App = lipgloss.NewStyle().Foreground(t.Fg).Background(t.Bg)
	t.Header = lipgloss.NewStyle().Padding(0, 1).Background(t.Bg)
	t.Brand = lipgloss.NewStyle().Bold(true).Foreground(t.Accent).Background(t.Bg)
	t.Subtitle = lipgloss.NewStyle().Foreground(t.Muted).Background(t.Bg)
	t.TabActive = lipgloss.NewStyle().
		Bold(true).
		Foreground(colPepper).
		Background(t.Accent).
		Padding(0, 2)
	t.TabIdle = lipgloss.NewStyle().
		Foreground(t.Muted).
		Background(colPanel).
		Padding(0, 2)
	t.Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Background(colPanel).
		Padding(0, 1)
	t.PanelFocus = t.Panel.BorderForeground(t.BorderFocus)
	t.Section = lipgloss.NewStyle().Bold(true).Foreground(t.AccentAlt).MarginTop(1)
	t.Label = lipgloss.NewStyle().Foreground(t.Muted).Width(18)
	t.Value = lipgloss.NewStyle().Foreground(t.Fg)
	t.ChipOn = lipgloss.NewStyle().
		Bold(true).
		Foreground(colPepper).
		Background(t.Good).
		Padding(0, 1).
		MarginRight(1)
	t.ChipOff = lipgloss.NewStyle().
		Foreground(t.Muted).
		Background(lipgloss.Color("#2A2228")).
		Padding(0, 1).
		MarginRight(1)
	t.Help = lipgloss.NewStyle().Foreground(t.Subtle)
	t.Status = lipgloss.NewStyle().Foreground(t.Muted)
	t.Error = lipgloss.NewStyle().Foreground(t.Bad).Bold(true)
	t.Title = lipgloss.NewStyle().Bold(true).Foreground(t.Fg)
	t.Desc = lipgloss.NewStyle().Foreground(t.Muted)
	t.Stat = lipgloss.NewStyle().Foreground(t.AccentAlt).Bold(true)
	t.Rule = lipgloss.NewStyle().Foreground(t.Border)

	return t
}

func (t Theme) chip(label string, on bool) string {
	if on {
		return t.ChipOn.Render(label)
	}
	return t.ChipOff.Render(label)
}

// gradientText paints each character cycling through accent colors.
func gradientText(s string, at time.Time) string {
	palette := []color.Color{
		colAccent, colBorderHot, colAccentAlt, colInfo, colGood, colWarn, colAccent,
	}
	shift := int(at.UnixNano()/int64(80*time.Millisecond)) % len(palette)
	var b strings.Builder
	i := 0
	for _, r := range s {
		if r == '\n' {
			b.WriteByte('\n')
			continue
		}
		if r == ' ' {
			b.WriteByte(' ')
			i++
			continue
		}
		c := palette[(i+shift)%len(palette)]
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(c).Render(string(r)))
		i++
	}
	return b.String()
}

func sparkleRow(width int, at time.Time) string {
	if width < 8 {
		width = 8
	}
	glyphs := []string{"·", "✦", "✧", "·", "⋆", "·"}
	colors := []color.Color{colSubtle, colAccent, colAccentAlt, colBorderHot, colInfo, colSubtle}
	phase := int(at.UnixNano()/int64(120*time.Millisecond)) % len(glyphs)
	var parts []string
	for i := 0; i < min(width, 48); i++ {
		g := glyphs[(i+phase)%len(glyphs)]
		c := colors[(i+phase)%len(colors)]
		parts = append(parts, lipgloss.NewStyle().Foreground(c).Render(g))
	}
	return strings.Join(parts, " ")
}

func accentBar(width int, at time.Time) string {
	if width < 10 {
		width = 10
	}
	palette := []color.Color{colAccent, colBorderHot, colAccentAlt, colInfo}
	phase := int(at.UnixNano()/int64(90*time.Millisecond)) % len(palette)
	var b strings.Builder
	for i := 0; i < width; i++ {
		c := palette[(i+phase)%len(palette)]
		b.WriteString(lipgloss.NewStyle().Foreground(c).Render("▀"))
	}
	return b.String()
}

func fmtStat(n int, label string) string {
	return lipgloss.NewStyle().Foreground(colAccent).Bold(true).Render(fmt.Sprintf("%d", n)) +
		lipgloss.NewStyle().Foreground(colMuted).Render(" "+label)
}
