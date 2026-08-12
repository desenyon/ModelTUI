package ui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/desenyon/ModelTUI/internal/catalog"
	"github.com/desenyon/ModelTUI/internal/format"
)

type itemKind int

const (
	kindModel itemKind = iota
	kindProvider
	kindOffering
	kindLab
)

type browseItem struct {
	kind     itemKind
	title    string
	desc     string
	filter   string
	model    *catalog.CanonicalModel
	provider *catalog.Provider
	offering *catalog.Offering
	lab      *catalog.Lab
}

func (i browseItem) Title() string       { return i.title }
func (i browseItem) Description() string { return i.desc }
func (i browseItem) FilterValue() string { return i.filter }

func modelItems(models []catalog.CanonicalModel) []list.Item {
	items := make([]list.Item, 0, len(models))
	for i := range models {
		m := models[i]
		chips := capabilityShort(m.Reasoning, m.ToolCall, m.Attachment, m.OpenWeights, m.StructuredOutput)
		desc := format.Join(m.ID, m.Family, format.LimitLine(m.Limit), chips)
		items = append(items, browseItem{
			kind:   kindModel,
			title:  m.Name,
			desc:   desc,
			filter: strings.Join([]string{m.Name, m.ID, m.Family, m.Description, m.Knowledge}, " "),
			model:  &models[i],
		})
	}
	return items
}

func providerItems(providers []catalog.Provider) []list.Item {
	items := make([]list.Item, 0, len(providers))
	for i := range providers {
		p := providers[i]
		desc := format.Join(p.ID, fmt.Sprintf("%d models", len(p.Models)), p.NPM, p.API)
		items = append(items, browseItem{
			kind:     kindProvider,
			title:    p.Name,
			desc:     desc,
			filter:   strings.Join([]string{p.Name, p.ID, p.NPM, p.API, strings.Join(p.Env, " ")}, " "),
			provider: &providers[i],
		})
	}
	return items
}

func offeringItems(offerings []catalog.Offering) []list.Item {
	items := make([]list.Item, 0, len(offerings))
	for i := range offerings {
		o := offerings[i]
		m := o.Model
		price := "n/a"
		if m.Cost != nil {
			price = fmt.Sprintf("%s/%s", format.Money(m.Cost.Input), format.Money(m.Cost.Output))
		}
		status := m.Status
		if status == "" {
			status = "stable"
		}
		chips := capabilityShort(m.Reasoning, m.ToolCall, m.Attachment, m.OpenWeights, m.StructuredOutput)
		desc := format.Join(o.ProviderName, m.ID, format.Tokens(m.Limit.Context), price, status, chips)
		items = append(items, browseItem{
			kind:     kindOffering,
			title:    m.Name,
			desc:     desc,
			filter:   strings.Join([]string{m.Name, m.ID, o.ProviderID, o.ProviderName, m.Family, m.Description, m.Status}, " "),
			offering: &offerings[i],
		})
	}
	return items
}

func labItems(labs []catalog.Lab) []list.Item {
	items := make([]list.Item, 0, len(labs))
	for i := range labs {
		l := labs[i]
		desc := format.Join(l.ID, fmt.Sprintf("%d canonical models", len(l.Models)), catalog.LogoURL("lab", l.ID))
		items = append(items, browseItem{
			kind:   kindLab,
			title:  l.Name,
			desc:   desc,
			filter: strings.Join([]string{l.Name, l.ID}, " "),
			lab:    &labs[i],
		})
	}
	return items
}

func capabilityShort(reasoning, tools, attach, openWeights bool, structured *bool) string {
	parts := make([]string, 0, 5)
	if reasoning {
		parts = append(parts, "R")
	}
	if tools {
		parts = append(parts, "T")
	}
	if attach {
		parts = append(parts, "A")
	}
	if openWeights {
		parts = append(parts, "OW")
	}
	if structured != nil && *structured {
		parts = append(parts, "SO")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "")
}

type itemDelegate struct {
	theme  Theme
	styles list.DefaultItemStyles
}

func newItemDelegate(theme Theme) itemDelegate {
	styles := list.NewDefaultItemStyles(true)
	styles.NormalTitle = styles.NormalTitle.Foreground(theme.Fg).Padding(0, 0, 0, 2)
	styles.NormalDesc = styles.NormalDesc.Foreground(theme.Muted).Padding(0, 0, 0, 2)
	styles.SelectedTitle = styles.SelectedTitle.
		Foreground(theme.Accent).
		BorderForeground(theme.Accent).
		Padding(0, 0, 0, 1)
	styles.SelectedDesc = styles.SelectedDesc.
		Foreground(theme.AccentAlt).
		BorderForeground(theme.Accent).
		Padding(0, 0, 0, 1)
	styles.DimmedTitle = styles.DimmedTitle.Foreground(theme.Subtle)
	styles.DimmedDesc = styles.DimmedDesc.Foreground(theme.Subtle)
	styles.FilterMatch = lipgloss.NewStyle().Foreground(theme.AccentAlt).Underline(true)
	return itemDelegate{theme: theme, styles: styles}
}

func (d itemDelegate) Height() int                               { return 2 }
func (d itemDelegate) Spacing() int                              { return 1 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd   { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(browseItem)
	if !ok {
		return
	}
	selected := index == m.Index()

	titleStyle := d.styles.NormalTitle
	descStyle := d.styles.NormalDesc
	if selected {
		titleStyle = d.styles.SelectedTitle
		descStyle = d.styles.SelectedDesc
	}

	title := titleStyle.Render(i.title)
	desc := descStyle.Render(truncate(i.desc, m.Width()-4))
	fmt.Fprintf(w, "%s\n%s", title, desc)
}

func truncate(s string, width int) string {
	if width <= 1 || lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if len(runes) <= 1 {
		return s
	}
	for len(runes) > 0 && lipgloss.Width(string(runes)) > width-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
