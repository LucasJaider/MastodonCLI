package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mastodoncli/internal/mastodon"
	"mastodoncli/internal/metrics"
	"mastodoncli/internal/ui/components"
)

var (
	metricsFollowStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("70"))
	metricsLikeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	metricsBoostStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	metricsSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
)

type metricsView struct {
	list           list.Model
	detail         viewport.Model
	series         []metrics.DailyMetric
	loading        bool
	selected       int
	rangeDays      int
	progressDone   int
	progressTotal  int
	progressActive bool
	progressCh     <-chan metricsProgressMsg
}

type metricsMsg struct {
	series []metrics.DailyMetric
}

type metricsProgressMsg struct {
	done     int
	total    int
	complete bool
}

type metricsItem struct {
	title   string
	snippet string
}

func (m metricsItem) Title() string       { return m.title }
func (m metricsItem) Description() string { return m.snippet }
func (m metricsItem) FilterValue() string { return m.title + " " + m.snippet }

func newMetricsView(title string) *metricsView {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("86"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("86"))
	delegate.SetHeight(2)

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = title
	l.SetShowHelp(true)
	l.SetFilteringEnabled(false)
	l.SetShowStatusBar(true)
	l.SetShowPagination(true)
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "last 7 days")),
			key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "last 30 days")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
			key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
			key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		}
	}
	l.DisableQuitKeybindings()

	vp := viewport.New(0, 0)
	vp.YPosition = 0

	return &metricsView{
		list:      l,
		detail:    vp,
		loading:   true,
		rangeDays: 7,
	}
}

func (m *model) ensureMetricsLoaded() tea.Cmd {
	view := m.metricsView
	if !view.loading && len(view.series) > 0 {
		return nil
	}
	view.loading = true
	view.progressActive = true
	view.progressDone = 0
	view.progressTotal = 0
	view.list.Title = fmt.Sprintf("Metrics (%dd)", view.rangeDays)
	view.list.SetItems([]list.Item{loadingItem("Loading metrics...", "Scanning groups...")})
	view.list.StartSpinner()
	progressCh := make(chan metricsProgressMsg, 4)
	view.progressCh = progressCh
	return tea.Batch(
		fetchMetricsCmd(m.client, view.rangeDays, progressCh),
		listenMetricsProgressCmd(progressCh),
		m.spinner.Tick,
	)
}

func (m *model) switchMetricsRange(days int) (tea.Model, tea.Cmd) {
	view := m.metricsView
	if view.rangeDays == days && len(view.series) > 0 {
		return m, nil
	}
	view.rangeDays = days
	view.loading = true
	view.progressActive = true
	view.progressDone = 0
	view.progressTotal = 0
	view.list.Title = fmt.Sprintf("Metrics (%dd)", view.rangeDays)
	view.list.SetItems([]list.Item{loadingItem("Loading metrics...", "Scanning groups...")})
	view.list.StartSpinner()
	progressCh := make(chan metricsProgressMsg, 4)
	view.progressCh = progressCh
	return m, tea.Batch(
		fetchMetricsCmd(m.client, view.rangeDays, progressCh),
		listenMetricsProgressCmd(progressCh),
		m.spinner.Tick,
	)
}

func fetchMetricsCmd(client *mastodon.Client, days int, progressCh chan<- metricsProgressMsg) tea.Cmd {
	return func() tea.Msg {
		series, err := metrics.FetchDailyMetrics(client, days, func(scanned int) {
			progressCh <- metricsProgressMsg{done: scanned}
		})
		close(progressCh)
		if err != nil {
			return feedErrMsg{tab: tabMetrics, err: err}
		}
		return metricsMsg{series: series}
	}
}

func listenMetricsProgressCmd(progressCh <-chan metricsProgressMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-progressCh
		if !ok {
			return metricsProgressMsg{complete: true}
		}
		return msg
	}
}

func (m *model) setMetrics(view *metricsView, series []metrics.DailyMetric) {
	view.series = series
	items := make([]list.Item, 0, components.Max(1, len(series)))
	if len(series) == 0 {
		items = append(items, emptyItem("No metrics", "Nothing to show here yet."))
	} else {
		for _, item := range series {
			items = append(items, metricsToItem(item))
		}
	}
	view.list.SetItems(items)
	view.list.Title = fmt.Sprintf("Metrics (%dd)", view.rangeDays)
}

func (m *model) renderMetricsDetail(view *metricsView) {
	if view.detail.Width == 0 {
		return
	}
	if len(view.series) == 0 {
		if view.loading {
			view.detail.SetContent(renderMetricsLoading(view, m.spinner.View()))
		} else {
			view.detail.SetContent("No metrics yet.")
		}
		return
	}

	selected := view.list.Index()
	if selected < 0 || selected >= len(view.series) {
		selected = 0
	}
	view.detail.SetContent(renderMetricsChart(view.series, view.detail.Width, selected))
}

func (m model) renderMetrics(view *metricsView) string {
	leftWidth := components.Max(30, m.width/2)
	rightWidth := m.width - leftWidth - 1
	if rightWidth < 20 {
		return view.list.View()
	}

	left := lipgloss.NewStyle().Width(leftWidth).Height(m.contentHeight()).Render(view.list.View())
	right := lipgloss.NewStyle().Width(rightWidth).Height(m.contentHeight()).Render(view.detail.View())
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

func (m *model) resizeMetrics(view *metricsView) {
	if m.width == 0 || m.height == 0 {
		return
	}

	leftWidth := components.Max(30, m.width/2)
	rightWidth := m.width - leftWidth - 1
	view.list.SetSize(leftWidth, components.Max(5, m.contentHeight()))
	view.detail.Width = rightWidth
	view.detail.Height = components.Max(5, m.contentHeight())
}

func metricsToItem(item metrics.DailyMetric) metricsItem {
	title := item.Label
	snippet := fmt.Sprintf("F %d · L %d · B %d", item.Follows, item.Likes, item.Boosts)
	return metricsItem{title: title, snippet: snippet}
}

func renderMetricsLoading(view *metricsView, spinnerView string) string {
	var builder strings.Builder
	builder.WriteString(spinnerView)
	builder.WriteString(" Loading metrics")
	builder.WriteString(fmt.Sprintf(" (%dd)...\n", view.rangeDays))
	if view.progressActive {
		if view.progressTotal > 0 {
			builder.WriteString(fmt.Sprintf("Scanned %d/%d groups...", view.progressDone, view.progressTotal))
		} else if view.progressDone > 0 {
			builder.WriteString(fmt.Sprintf("Scanned %d groups...", view.progressDone))
		} else {
			builder.WriteString("Scanning groups...")
		}
	}
	return builder.String()
}

func renderMetricsChart(series []metrics.DailyMetric, width int, selected int) string {
	header := metrics.FormatTotal(series)
	lines := make([]string, 0, len(series)+6)
	lines = append(lines, header)
	lines = append(lines, renderMetricsLegend())
	lines = append(lines, renderMetricsSparkline(series, width))

	if selected < 0 || selected >= len(series) {
		selected = 0
	}
	selectedDay := series[selected]
	lines = append(lines, renderMetricsSelection(selectedDay, series, selected))
	lines = append(lines, "")

	labelWidth := 6
	countsTemplate := fmt.Sprintf("F%d L%d B%d", maxCount(series), maxCount(series), maxCount(series))
	barWidth := width - (labelWidth + 1 + len(countsTemplate) + 3)
	barWidth = components.Max(10, barWidth)
	maxTotal := maxTotal(series)
	if maxTotal == 0 {
		maxTotal = 1
	}

	for _, day := range series {
		counts := fmt.Sprintf("F%d L%d B%d", day.Follows, day.Likes, day.Boosts)
		bar := renderStackedBar(day, maxTotal, barWidth)
		line := fmt.Sprintf("%-6s %s %s", day.Label, bar, counts)
		if selected >= 0 && selected < len(series) && day.Date.Equal(series[selected].Date) {
			line = metricsSelectedStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m model) renderMetricsRanges() string {
	labels := []string{"7d", "30d"}
	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		style := components.TabStyle
		if (label == "7d" && m.metricsView.rangeDays == 7) || (label == "30d" && m.metricsView.rangeDays == 30) {
			style = components.TabActiveStyle
		}
		parts = append(parts, components.RenderTabLabel(label, style))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func renderMetricsLegend() string {
	parts := []string{
		metricsFollowStyle.Render("Follows ##"),
		metricsLikeStyle.Render("Likes ##"),
		metricsBoostStyle.Render("Boosts ##"),
	}
	return strings.Join(parts, "  ")
}

func renderMetricsSelection(selected metrics.DailyMetric, series []metrics.DailyMetric, selectedIndex int) string {
	total := selected.Follows + selected.Likes + selected.Boosts
	pctF, pctL, pctB := 0, 0, 0
	if total > 0 {
		pctF = (selected.Follows * 100) / total
		pctL = (selected.Likes * 100) / total
		pctB = (selected.Boosts * 100) / total
	}
	line := fmt.Sprintf("Selected %s  F%d L%d B%d  Pct F%d%% L%d%% B%d%%",
		selected.Label,
		selected.Follows,
		selected.Likes,
		selected.Boosts,
		pctF,
		pctL,
		pctB,
	)
	if selectedIndex <= 0 {
		return line + "  Δ n/a"
	}
	prev := series[selectedIndex-1]
	deltaF := selected.Follows - prev.Follows
	deltaL := selected.Likes - prev.Likes
	deltaB := selected.Boosts - prev.Boosts
	return fmt.Sprintf("%s  Δ F%+d L%+d B%+d", line, deltaF, deltaL, deltaB)
}

func renderMetricsSparkline(series []metrics.DailyMetric, width int) string {
	const label = "Trend "
	if len(series) == 0 {
		return label + "-"
	}
	maxWidth := width - len(label) - 1
	if maxWidth < 4 {
		maxWidth = 4
	}
	points := series
	if len(points) > maxWidth {
		step := (len(points) + maxWidth - 1) / maxWidth
		trimmed := make([]metrics.DailyMetric, 0, maxWidth)
		for i := 0; i < len(points); i += step {
			trimmed = append(trimmed, points[i])
		}
		points = trimmed
	}

	maxTotal := maxTotal(points)
	if maxTotal == 0 {
		maxTotal = 1
	}
	ramp := " .:-=+*#"
	var builder strings.Builder
	builder.WriteString(label)
	for _, day := range points {
		total := day.Follows + day.Likes + day.Boosts
		index := (total * (len(ramp) - 1)) / maxTotal
		if index < 0 {
			index = 0
		}
		if index >= len(ramp) {
			index = len(ramp) - 1
		}
		builder.WriteByte(ramp[index])
	}
	return builder.String()
}

func renderStackedBar(day metrics.DailyMetric, maxTotal, width int) string {
	if width <= 0 {
		return ""
	}
	total := day.Follows + day.Likes + day.Boosts
	if total == 0 {
		return strings.Repeat(" ", width)
	}

	segments := []int{day.Follows, day.Likes, day.Boosts}
	lengths := scaledSegments(segments, maxTotal, width)
	var builder strings.Builder
	builder.Grow(width)
	builder.WriteString(metricsFollowStyle.Render(strings.Repeat("#", lengths[0])))
	builder.WriteString(metricsLikeStyle.Render(strings.Repeat("#", lengths[1])))
	builder.WriteString(metricsBoostStyle.Render(strings.Repeat("#", lengths[2])))
	filled := lengths[0] + lengths[1] + lengths[2]
	if filled < width {
		builder.WriteString(strings.Repeat(" ", width-filled))
	}
	return builder.String()
}

func scaledSegments(values []int, maxTotal, width int) []int {
	if width <= 0 {
		return []int{0, 0, 0}
	}
	total := values[0] + values[1] + values[2]
	if total == 0 {
		return []int{0, 0, 0}
	}

	lengths := make([]int, 3)
	for i, value := range values {
		lengths[i] = (value * width) / maxTotal
	}

	ensureMinimumSegments(values, lengths, width)
	normalizeSegments(lengths, width)
	return lengths
}

func ensureMinimumSegments(values, lengths []int, width int) {
	for i, value := range values {
		if value > 0 && lengths[i] == 0 && width > 0 {
			lengths[i] = 1
		}
	}
}

func normalizeSegments(lengths []int, width int) {
	total := lengths[0] + lengths[1] + lengths[2]
	if total == width {
		return
	}
	for total > width {
		index := maxIndex(lengths)
		if lengths[index] == 0 {
			break
		}
		lengths[index]--
		total--
	}
	for total < width {
		index := maxIndex(lengths)
		lengths[index]++
		total++
	}
}

func maxIndex(values []int) int {
	maxIdx := 0
	for i, value := range values {
		if value > values[maxIdx] {
			maxIdx = i
		}
	}
	return maxIdx
}

func maxCount(series []metrics.DailyMetric) int {
	maxValue := 0
	for _, day := range series {
		if day.Follows > maxValue {
			maxValue = day.Follows
		}
		if day.Likes > maxValue {
			maxValue = day.Likes
		}
		if day.Boosts > maxValue {
			maxValue = day.Boosts
		}
	}
	return maxValue
}

func maxTotal(series []metrics.DailyMetric) int {
	maxValue := 0
	for _, day := range series {
		total := day.Follows + day.Likes + day.Boosts
		if total > maxValue {
			maxValue = total
		}
	}
	return maxValue
}
