package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone"

	"github.com/desenyon/ModelTUI/internal/catalog"
)

type tabID int

const (
	tabModels tabID = iota
	tabProviders
	tabOfferings
	tabLabs
)

func (t tabID) String() string {
	switch t {
	case tabModels:
		return "Models"
	case tabProviders:
		return "Providers"
	case tabOfferings:
		return "Offerings"
	case tabLabs:
		return "Labs"
	default:
		return ""
	}
}

type focusPane int

const (
	focusList focusPane = iota
	focusDetail
)

type loadMsg struct {
	index       *catalog.Index
	source      string
	err         error
	silent      bool
	notModified bool
	retryAfter  time.Duration
}

type autoRefreshMsg struct{}

type model struct {
	theme   Theme
	width   int
	height  int
	ready   bool
	loading bool
	err     string
	source  string
	index   *catalog.Index
	client  *catalog.Client

	tab    tabID
	focus  focusPane
	lists  [4]list.Model
	detail viewport.Model
	help   help.Model
	keys   keyMap

	spin     spinner.Model
	progress progress.Model
	pct      float64

	logoSpring  SpringPair
	panelSpring SpringPair
	pulseAt     time.Time

	showHelp bool
	status   string
	zones    *zone.Manager

	capsSelected []string
	filterOpen   bool
	filterForm   *huh.Form
	refreshing   bool
}

type keyMap struct {
	Quit       key.Binding
	TabNext    key.Binding
	TabPrev    key.Binding
	Focus      key.Binding
	Help       key.Binding
	Refresh    key.Binding
	Filter     key.Binding
	CapsFilter key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Refresh, k.TabNext, k.Focus, k.Filter, k.CapsFilter, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Refresh, k.TabNext, k.TabPrev, k.Focus},
		{k.Filter, k.CapsFilter, k.Help, k.Quit},
	}
}

func newKeyMap() keyMap {
	return keyMap{
		Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		TabNext:    key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "next tab")),
		TabPrev:    key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("shift+tab", "prev tab")),
		Focus:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "focus detail")),
		Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Refresh:    key.NewBinding(key.WithKeys("space", "ctrl+r"), key.WithHelp("space", "refresh API")),
		Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "fuzzy filter")),
		CapsFilter: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "capability filters")),
	}
}

// New creates the root Bubble Tea model.
func New() tea.Model {
	theme := NewTheme()
	z := zone.New()

	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = lipgloss.NewStyle().Foreground(theme.Accent)

	pr := progress.New(
		progress.WithDefaultBlend(),
		progress.WithWidth(42),
		progress.WithColors(colAccent, colBorderHot, colAccentAlt),
	)

	delegate := newItemDelegate(theme)
	mkList := func(title string) list.Model {
		l := list.New(nil, delegate, 20, 10)
		l.Title = title
		l.SetShowStatusBar(true)
		l.SetFilteringEnabled(true)
		l.SetShowHelp(false)
		l.DisableQuitKeybindings()
		l.Styles = list.DefaultStyles(true)
		l.Styles.Title = lipgloss.NewStyle().
			Background(theme.Accent).
			Foreground(theme.Bg).
			Bold(true).
			Padding(0, 1)
		prompt := lipgloss.NewStyle().Foreground(theme.Accent)
		l.Styles.Filter.Focused.Prompt = prompt
		l.Styles.Filter.Blurred.Prompt = prompt
		l.Styles.Filter.Cursor.Color = theme.AccentAlt
		l.Styles.StatusBar = lipgloss.NewStyle().Foreground(theme.Muted)
		l.Styles.DividerDot = lipgloss.NewStyle().Foreground(theme.Subtle).SetString(" • ")
		l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(theme.Subtle)
		return l
	}

	h := help.New()
	h.Styles = help.DefaultStyles(true)
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(theme.Accent)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(theme.Muted)
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(theme.Subtle)
	h.Styles.FullKey = h.Styles.ShortKey
	h.Styles.FullDesc = h.Styles.ShortDesc
	h.Styles.FullSeparator = h.Styles.ShortSeparator

	m := model{
		theme:       theme,
		loading:     true,
		tab:         tabModels,
		focus:       focusList,
		spin:        sp,
		progress:    pr,
		help:        h,
		keys:        newKeyMap(),
		logoSpring:  newSpring(8.0, 0.45),
		panelSpring: newSpring(10.0, 0.7),
		pulseAt:     time.Now(),
		zones:       z,
		client:      catalog.NewClient(),
		status:      "Fetching models.dev catalog…",
	}
	m.lists[tabModels] = mkList("Canonical models")
	m.lists[tabProviders] = mkList("Providers")
	m.lists[tabOfferings] = mkList("Provider offerings")
	m.lists[tabLabs] = mkList("Labs")
	m.detail = viewport.New(viewport.WithWidth(40), viewport.WithHeight(10))
	m.logoSpring.snap(-8)
	m.logoSpring.setTarget(0)
	m.panelSpring.snap(38)
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spin.Tick,
		m.bootstrapCatalog(),
		tickAnim(),
		m.progress.SetPercent(0.08),
		scheduleAutoRefresh(catalog.AutoRefreshEvery),
	)
}

func scheduleAutoRefresh(d time.Duration) tea.Cmd {
	if d < time.Second {
		d = time.Second
	}
	return tea.Tick(d, func(time.Time) tea.Msg { return autoRefreshMsg{} })
}

func (m model) bootstrapCatalog() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		client := catalog.NewClient()
		cat, source, err := client.LoadCatalog(ctx)
		if err != nil {
			return loadMsg{err: err}
		}
		return loadMsg{index: catalog.BuildIndex(cat, source), source: source}
	}
}

func (m model) refreshCatalog(force, silent bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		client := catalog.NewClient()
		res := client.RefreshCatalog(ctx, force)
		if res.Err != nil {
			return loadMsg{err: res.Err, silent: silent, retryAfter: res.RetryAfter}
		}
		if res.NotModified && res.Catalog == nil {
			return loadMsg{silent: silent, notModified: true, source: res.Source}
		}
		return loadMsg{
			index:       catalog.BuildIndex(res.Catalog, res.Source),
			source:      res.Source,
			silent:      silent,
			notModified: res.NotModified,
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		if m.filterOpen && m.filterForm != nil {
			var cmd tea.Cmd
			form, cmd := m.filterForm.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.filterForm = f
			}
			cmds = append(cmds, cmd)
		}
		m.refreshDetail()
		return m, tea.Batch(cmds...)

	case frameMsg:
		m.pulseAt = time.Time(msg)
		m.logoSpring.update()
		m.panelSpring.update()
		if m.loading && m.pct < 0.9 {
			m.pct += 0.004
			cmds = append(cmds, m.progress.SetPercent(m.pct))
		}
		cmds = append(cmds, tickAnim())
		return m, tea.Batch(cmds...)

	case loadMsg:
		m.refreshing = false
		if msg.err != nil {
			if msg.silent || m.index != nil {
				m.status = fmt.Sprintf("Refresh skipped: %v", msg.err)
				if msg.retryAfter > 0 {
					m.status = fmt.Sprintf("Rate limited — next try in %s", msg.retryAfter.Round(time.Second))
				}
				return m, scheduleAutoRefresh(m.client.NextAutoRefreshIn())
			}
			m.loading = false
			m.err = msg.err.Error()
			m.status = "Failed to load catalog"
			return m, nil
		}
		m.loading = false
		m.err = ""
		if msg.notModified && msg.index == nil {
			m.status = "Catalog already up to date"
			return m, scheduleAutoRefresh(m.client.NextAutoRefreshIn())
		}
		if msg.index != nil {
			m.index = msg.index
			m.source = msg.source
			m.status = fmt.Sprintf("Loaded %d models · %d providers · %d offerings · %d labs (%s)",
				len(msg.index.Models), len(msg.index.Providers), len(msg.index.Offerings), len(msg.index.Labs), msg.source)
			cmds = append(cmds, m.applyFilters()...)
			m.layout()
			m.refreshDetail()
		}
		if !msg.silent {
			cmds = append(cmds, m.progress.SetPercent(1))
		}
		cmds = append(cmds, scheduleAutoRefresh(m.client.NextAutoRefreshIn()))
		return m, tea.Batch(cmds...)

	case autoRefreshMsg:
		if m.loading || m.refreshing || m.filterOpen {
			return m, scheduleAutoRefresh(m.client.NextAutoRefreshIn())
		}
		if !m.client.ShouldAutoRefresh() {
			return m, scheduleAutoRefresh(m.client.NextAutoRefreshIn())
		}
		if ok, wait := m.client.CanRefresh(false); !ok {
			m.status = fmt.Sprintf("Auto-refresh waiting %s (rate limit spacing)", wait.Round(time.Second))
			return m, scheduleAutoRefresh(wait)
		}
		m.refreshing = true
		m.status = "Auto-refreshing models.dev…"
		return m, tea.Batch(m.refreshCatalog(false, true), scheduleAutoRefresh(catalog.AutoRefreshEvery))

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd

	case tea.MouseClickMsg:
		if m.loading || m.filterOpen {
			return m, nil
		}
		mouse := msg.Mouse()
		for i := 0; i < 4; i++ {
			z := m.zones.Get(fmt.Sprintf("tab-%d", i))
			if z == nil || z.IsZero() {
				continue
			}
			if mouse.X >= z.StartX && mouse.X <= z.EndX && mouse.Y >= z.StartY && mouse.Y <= z.EndY {
				m.tab = tabID(i)
				m.focus = focusList
				m.refreshDetail()
				return m, nil
			}
		}
		return m, nil
	}

	if m.filterOpen && m.filterForm != nil {
		form, cmd := m.filterForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.filterForm = f
		}
		switch m.filterForm.State {
		case huh.StateCompleted:
			m.filterOpen = false
			m.filterForm = nil
			cmds = append(cmds, cmd)
			cmds = append(cmds, m.applyFilters()...)
			m.refreshDetail()
			m.status = fmt.Sprintf("Filters: %s", filterSummary(m.capsSelected))
			return m, tea.Batch(cmds...)
		case huh.StateAborted:
			m.filterOpen = false
			m.filterForm = nil
			return m, cmd
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.loading {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		// When fuzzy-filtering, let the list own most keys.
		if m.focus == focusList && m.lists[m.tab].FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			m.layout()
			return m, nil
		case key.Matches(msg, m.keys.CapsFilter):
			m.filterOpen = true
			m.filterForm = newFilterForm(&m.capsSelected)
			return m, m.filterForm.Init()
		case key.Matches(msg, m.keys.Refresh):
			if m.refreshing {
				m.status = "Refresh already in flight…"
				return m, nil
			}
			if ok, wait := m.client.CanRefresh(true); !ok {
				m.status = fmt.Sprintf("Rate limit spacing — try again in %s", wait.Round(time.Second))
				return m, nil
			}
			m.refreshing = true
			m.status = "Refreshing models.dev (rate-limit aware)…"
			return m, tea.Batch(m.spin.Tick, m.refreshCatalog(true, false))
		case key.Matches(msg, m.keys.TabNext):
			m.tab = (m.tab + 1) % 4
			m.focus = focusList
			m.refreshDetail()
			return m, nil
		case key.Matches(msg, m.keys.TabPrev):
			m.tab = (m.tab + 3) % 4
			m.focus = focusList
			m.refreshDetail()
			return m, nil
		case key.Matches(msg, m.keys.Focus):
			m.focus = focusDetail
			return m, nil
		case msg.String() == "1":
			m.tab = tabModels
			m.refreshDetail()
			return m, nil
		case msg.String() == "2":
			m.tab = tabProviders
			m.refreshDetail()
			return m, nil
		case msg.String() == "3":
			m.tab = tabOfferings
			m.refreshDetail()
			return m, nil
		case msg.String() == "4":
			m.tab = tabLabs
			m.refreshDetail()
			return m, nil
		case msg.String() == "esc":
			if m.focus == focusDetail {
				m.focus = focusList
				return m, nil
			}
		}

		if m.focus == focusDetail {
			var cmd tea.Cmd
			m.detail, cmd = m.detail.Update(msg)
			return m, cmd
		}
	}

	if m.loading {
		return m, nil
	}

	if m.focus == focusList {
		prev := m.lists[m.tab].Index()
		var cmd tea.Cmd
		m.lists[m.tab], cmd = m.lists[m.tab].Update(msg)
		if m.lists[m.tab].Index() != prev {
			m.refreshDetail()
		}
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func filterSummary(selected []string) string {
	if len(selected) == 0 {
		return "none"
	}
	return strings.Join(selected, ", ")
}

func (m *model) applyFilters() []tea.Cmd {
	if m.index == nil {
		return nil
	}
	models := m.index.Models
	offerings := m.index.Offerings
	if len(m.capsSelected) > 0 {
		filteredModels := make([]catalog.CanonicalModel, 0, len(models))
		for _, model := range models {
			if matchCapsModel(model, m.capsSelected) {
				filteredModels = append(filteredModels, model)
			}
		}
		models = filteredModels

		filteredOfferings := make([]catalog.Offering, 0, len(offerings))
		for _, o := range offerings {
			if matchCapsOffering(o, m.capsSelected) {
				filteredOfferings = append(filteredOfferings, o)
			}
		}
		offerings = filteredOfferings
	}
	return []tea.Cmd{
		m.lists[tabModels].SetItems(modelItems(models)),
		m.lists[tabProviders].SetItems(providerItems(m.index.Providers)),
		m.lists[tabOfferings].SetItems(offeringItems(offerings)),
		m.lists[tabLabs].SetItems(labItems(m.index.Labs)),
	}
}

func (m *model) layout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	header := 4
	footer := 2
	if m.showHelp {
		footer += 4
	}
	bodyH := max(6, m.height-header-footer)
	listW := m.width * 42 / 100
	if listW < 28 {
		listW = 28
	}
	if listW > m.width-24 {
		listW = max(20, m.width-24)
	}
	m.panelSpring.setTarget(float64(listW))
	detailW := max(20, m.width-listW-4)

	for i := range m.lists {
		m.lists[i].SetSize(listW-2, bodyH-2)
	}
	m.detail.SetWidth(detailW - 2)
	m.detail.SetHeight(bodyH - 2)
	m.progress.SetWidth(min(48, max(20, m.width-20)))
}

func (m *model) refreshDetail() {
	content := m.detailContent(max(20, m.detail.Width()))
	m.detail.SetContent(content)
	m.detail.GotoTop()
}

func (m model) selectedItem() (browseItem, bool) {
	item := m.lists[m.tab].SelectedItem()
	if item == nil {
		return browseItem{}, false
	}
	bi, ok := item.(browseItem)
	return bi, ok
}

func (m model) View() tea.View {
	var body string
	if !m.ready {
		body = m.theme.Brand.Render("Booting ModelTUI...")
	} else if m.loading {
		body = m.viewSplash()
	} else if m.err != "" && m.index == nil {
		body = m.theme.Error.Render("Error: "+m.err) + "\n" + m.theme.Help.Render("q quit")
	} else if m.filterOpen && m.filterForm != nil {
		panel := m.theme.PanelFocus.Width(min(72, m.width-4)).Render(m.filterForm.View())
		body = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel,
			lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(m.theme.Bg)))
	} else {
		body = m.viewMain()
	}

	// Paint full canvas background so the UI never looks washed-out.
	body = lipgloss.NewStyle().
		Width(max(1, m.width)).
		Height(max(1, m.height)).
		Background(m.theme.Bg).
		Foreground(m.theme.Fg).
		Render(body)

	v := tea.NewView(m.zones.Scan(body))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "ModelTUI · models.dev"
	v.BackgroundColor = m.theme.Bg
	v.ForegroundColor = m.theme.Fg
	return v
}

func (m model) viewSplash() string {
	logoY := m.logoSpring.intPos()
	pad := strings.Repeat("\n", max(0, 1+logoY))
	logo := gradientText(splashLogo, m.pulseAt)
	sub := lipgloss.NewStyle().Foreground(colAccentAlt).Italic(true).
		Render("The glamorous models.dev explorer")
	spin := m.spin.View() + "  " + lipgloss.NewStyle().Foreground(colAccent).Render(m.status)
	bar := m.progress.View()
	ray := sparkleRow(min(40, max(12, m.width/3)), m.pulseAt)
	rule := accentBar(min(56, max(24, m.width/2)), m.pulseAt)

	block := lipgloss.JoinVertical(lipgloss.Center, logo, "", sub, "", ray, "", rule, "", spin, "", bar)
	return pad + lipgloss.Place(m.width, max(10, m.height-2), lipgloss.Center, lipgloss.Center, block,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(m.theme.Bg)))
}

func (m model) viewMain() string {
	header := m.viewHeader()
	tabs := m.viewTabs()
	listW := m.panelSpring.intPos()
	if listW < 20 {
		listW = 20
	}
	detailW := max(20, m.width-listW-2)
	bodyH := max(6, m.height-lipgloss.Height(header)-lipgloss.Height(tabs)-4)
	if m.showHelp {
		bodyH = max(6, bodyH-4)
	}

	listStyle := m.theme.Panel
	detailStyle := m.theme.Panel
	if m.focus == focusList {
		listStyle = m.theme.PanelFocus
	} else {
		detailStyle = m.theme.PanelFocus
	}

	listView := listStyle.Width(listW).Height(bodyH).Render(m.lists[m.tab].View())
	detailView := detailStyle.Width(detailW).Height(bodyH).Render(m.detail.View())
	row := lipgloss.JoinHorizontal(lipgloss.Top, listView, " ", detailView)

	footer := m.viewFooter()
	rule := accentBar(max(10, m.width-2), m.pulseAt)
	parts := []string{header, tabs, rule, row, footer}
	if m.showHelp {
		parts = []string{header, tabs, rule, row, m.help.View(m.keys), footer}
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) viewHeader() string {
	brand := m.theme.Brand.Render("◆ ModelTUI")
	tag := lipgloss.NewStyle().Foreground(colAccentAlt).Render("models.dev")
	stats := ""
	if m.index != nil {
		stats = strings.Join([]string{
			fmtStat(len(m.index.Models), "models"),
			fmtStat(len(m.index.Providers), "providers"),
			fmtStat(len(m.index.Offerings), "offerings"),
			fmtStat(len(m.index.Labs), "labs"),
		}, lipgloss.NewStyle().Foreground(colSubtle).Render("  ·  "))
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top, brand, "  ", tag)
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(stats)-2)
	line := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), stats)
	return m.theme.Header.Width(m.width).Render(line)
}

func (m model) viewTabs() string {
	labels := []tabID{tabModels, tabProviders, tabOfferings, tabLabs}
	var parts []string
	for i, t := range labels {
		label := fmt.Sprintf("%d %s", i+1, t.String())
		style := m.theme.TabIdle
		if t == m.tab {
			style = m.theme.TabActive
			// Pulse the active tab border glow via alternating bold/normal feel.
			if int(m.pulseAt.UnixNano()/int64(400*time.Millisecond))%2 == 0 {
				style = style.Underline(true)
			}
		}
		parts = append(parts, m.zones.Mark(fmt.Sprintf("tab-%d", t), style.Render(label)))
	}
	return " " + lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m model) viewFooter() string {
	focus := "list"
	if m.focus == focusDetail {
		focus = "detail"
	}
	refreshHint := lipgloss.NewStyle().Foreground(colAccent).Bold(true).Render("space") +
		lipgloss.NewStyle().Foreground(colMuted).Render(":refresh")
	if m.refreshing {
		refreshHint = lipgloss.NewStyle().Foreground(colWarn).Bold(true).Render("refreshing...")
	}
	meta := lipgloss.NewStyle().Foreground(colSubtle).Render(
		fmt.Sprintf("  focus:%s  source:%s", focus, orDash(m.source)),
	)
	left := refreshHint + meta
	right := m.help.View(m.keys)
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2)
	status := ""
	if m.status != "" {
		status = "\n " + lipgloss.NewStyle().Foreground(colAccentAlt).Render(truncate(m.status, m.width-4))
	}
	return " " + left + strings.Repeat(" ", gap) + right + status
}

var splashLogo = strings.TrimRight(`
███╗   ███╗ ██████╗ ██████╗ ███████╗██╗     ████████╗██╗   ██╗██╗
████╗ ████║██╔═══██╗██╔══██╗██╔════╝██║     ╚══██╔══╝██║   ██║██║
██╔████╔██║██║   ██║██║  ██║█████╗  ██║        ██║   ██║   ██║██║
██║╚██╔╝██║██║   ██║██║  ██║██╔══╝  ██║        ██║   ██║   ██║██║
██║ ╚═╝ ██║╚██████╔╝██████╔╝███████╗███████╗   ██║   ╚██████╔╝██║
╚═╝     ╚═╝ ╚═════╝ ╚═════╝ ╚══════╝╚══════╝   ╚═╝    ╚═════╝ ╚═╝
`, "\n")
