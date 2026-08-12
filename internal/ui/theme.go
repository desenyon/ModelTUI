package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// Theme holds CharmTone-driven styles for ModelTUI.
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

	App       lipgloss.Style
	Header    lipgloss.Style
	Brand     lipgloss.Style
	Subtitle  lipgloss.Style
	TabActive lipgloss.Style
	TabIdle   lipgloss.Style
	Panel     lipgloss.Style
	PanelFocus lipgloss.Style
	Section   lipgloss.Style
	Label     lipgloss.Style
	Value     lipgloss.Style
	ChipOn    lipgloss.Style
	ChipOff   lipgloss.Style
	Help      lipgloss.Style
	Status    lipgloss.Style
	Error     lipgloss.Style
	Title     lipgloss.Style
	Desc      lipgloss.Style
}

// NewTheme builds the dark Charm-inspired palette.
func NewTheme() Theme {
	t := Theme{
		Bg:          charmtone.Pepper,
		Fg:          charmtone.Butter,
		Muted:       charmtone.Smoke,
		Subtle:      charmtone.Oyster,
		Accent:      charmtone.Cheeky,
		AccentAlt:   charmtone.Malibu,
		Good:        charmtone.Guac,
		Warn:        charmtone.Zest,
		Bad:         charmtone.Bengal,
		Info:        charmtone.Guppy,
		Border:      charmtone.Charcoal,
		BorderFocus: charmtone.Charple,
	}

	t.App = lipgloss.NewStyle().Foreground(t.Fg)
	t.Header = lipgloss.NewStyle().Padding(0, 1)
	t.Brand = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.Subtitle = lipgloss.NewStyle().Foreground(t.Muted)
	t.TabActive = lipgloss.NewStyle().
		Bold(true).
		Foreground(charmtone.Pepper).
		Background(t.Accent).
		Padding(0, 2)
	t.TabIdle = lipgloss.NewStyle().
		Foreground(t.Muted).
		Padding(0, 2)
	t.Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)
	t.PanelFocus = t.Panel.BorderForeground(t.BorderFocus)
	t.Section = lipgloss.NewStyle().Bold(true).Foreground(t.AccentAlt).MarginTop(1)
	t.Label = lipgloss.NewStyle().Foreground(t.Muted).Width(18)
	t.Value = lipgloss.NewStyle().Foreground(t.Fg)
	t.ChipOn = lipgloss.NewStyle().
		Foreground(charmtone.Pepper).
		Background(t.Good).
		Padding(0, 1).
		MarginRight(1)
	t.ChipOff = lipgloss.NewStyle().
		Foreground(t.Muted).
		Background(charmtone.Iron).
		Padding(0, 1).
		MarginRight(1)
	t.Help = lipgloss.NewStyle().Foreground(t.Subtle)
	t.Status = lipgloss.NewStyle().Foreground(t.Muted)
	t.Error = lipgloss.NewStyle().Foreground(t.Bad).Bold(true)
	t.Title = lipgloss.NewStyle().Bold(true).Foreground(t.Fg)
	t.Desc = lipgloss.NewStyle().Foreground(t.Muted)

	return t
}

func (t Theme) chip(label string, on bool) string {
	if on {
		return t.ChipOn.Render(label)
	}
	return t.ChipOff.Render(label)
}
