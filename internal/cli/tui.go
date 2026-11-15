package cli

import (
	"bytes"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/a-narquel/gh-snitch/internal/render"
	"github.com/a-narquel/gh-snitch/internal/stats"
)

// runTUI shows the arrow-key menu until the user quits.
//
// Bubble Tea follows "The Elm Architecture". You give it a model with three
// methods and it calls them in a loop:
//
//	Init   → a first action to run at start (we have none)
//	Update → gets an event (a key press, a window resize), changes the model
//	View   → draws the whole screen from the model, as one string
//
// You never draw anything yourself: change the model, and View redraws.
// It's like a Swing event listener, but with a single place that paints.
func runTUI(s *Session) error {
	// WithAltScreen draws on a separate screen, like vim or less do, so your
	// shell's scrollback is untouched when you quit.
	_, err := tea.NewProgram(&model{s: s, cursor: map[screen]int{}}, tea.WithAltScreen()).Run()
	return err
}

// screen is which part of the menu is showing.
type screen int

const (
	menuScreen    screen = iota // the main menu
	membersScreen               // pick a member
	filtersScreen               // period and excluded files
	periodScreen                // pick a period
	excludeScreen               // type a glob to exclude
	pageScreen                  // the output of a command, scrollable
)

// item is one line of a list: what it says and what Enter does.
type item struct {
	label string
	desc  string         // dim text after the label
	run   func() tea.Cmd // nil: the line can't be picked
}

type model struct {
	s      *Session
	screen screen
	cursor map[screen]int // each list remembers where its cursor was

	// pageScreen
	title  string
	lines  []string
	offset int    // first visible line
	back   screen // where Esc goes from the page

	input  string // excludeScreen's text box
	status string // a one-line message after an action

	width, height int // terminal size; 0 until the first WindowSizeMsg
}

func (m *model) Init() tea.Cmd { return nil }

// Update reacts to one event. Returning tea.Quit ends the program.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) { // a "type switch": branches on the event's type
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scrollPage(0) // keep the page offset valid after a resize
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.screen {
		case pageScreen:
			m.pageKey(msg)
			return m, nil
		case excludeScreen:
			m.inputKey(msg)
			return m, nil
		default:
			return m, m.listKey(msg)
		}
	}
	return m, nil
}

// ---- lists ----

// items returns the lines of the list on the current screen.
func (m *model) items() []item {
	s := m.s
	switch m.screen {
	case menuScreen:
		return []item{
			{"Leaderboard", "who did what: commits, lines, PRs, reviews", func() tea.Cmd {
				m.openPage("Leaderboard", cmdWho, nil)
				return nil
			}},
			{"Member", "one person's profile", func() tea.Cmd {
				m.goTo(membersScreen)
				return nil
			}},
			{"Activity", "commits over time", func() tea.Cmd {
				m.openPage("Activity", cmdActivity, nil)
				return nil
			}},
			{"Awards", "the fun stuff", func() tea.Cmd {
				m.openPage("Awards", cmdAwards, nil)
				return nil
			}},
			{"Filters", m.filterSummary(), func() tea.Cmd {
				m.goTo(filtersScreen)
				return nil
			}},
			{"Quit", "", func() tea.Cmd { return tea.Quit }},
		}

	case membersScreen:
		members := stats.Members(s.view())
		if len(members) == 0 {
			return []item{{label: render.Dim("no commits match the filters")}}
		}
		nameW := 0
		for _, mem := range members {
			nameW = max(nameW, render.Width(s.name(mem.Name)))
		}
		list := make([]item, len(members))
		for i, mem := range members {
			list[i] = item{
				label: render.Pad(s.name(mem.Name), nameW),
				desc:  render.Plural(mem.Commits, "commit"),
				run: func() tea.Cmd {
					m.openPage(mem.Name, cmdShow, []string{mem.Name})
					return nil
				},
			}
		}
		return list

	case filtersScreen:
		list := []item{
			{"Period", s.periodName(), func() tea.Cmd {
				m.goTo(periodScreen)
				return nil
			}},
			{"Exclude files", "ignore generated code, lock files, vendor/…", func() tea.Cmd {
				m.input = ""
				m.goTo(excludeScreen)
				return nil
			}},
		}
		for _, p := range s.filter.Exclude {
			list = append(list, item{"Stop excluding " + p, "", func() tea.Cmd {
				s.removeExclude(p)
				m.status = render.Green("✓ ") + "including " + p + " again"
				return nil
			}})
		}
		if !s.filter.IsZero() {
			list = append(list, item{"Clear all filters", "", func() tea.Cmd {
				s.filter = stats.Filter{} // the zero value means "no filters"
				m.status = render.Green("✓ ") + "filters cleared"
				return nil
			}})
		}
		return list

	case periodScreen:
		list := make([]item, len(periods))
		for i, p := range periods {
			mark := ""
			if p.label == s.filter.SinceLabel {
				mark = "•"
			}
			list[i] = item{p.name, mark, func() tea.Cmd {
				_ = s.setSince(p.label) // our own labels always parse
				m.status = render.Green("✓ ") + "period: " + strings.ToLower(p.name)
				m.screen = filtersScreen
				return nil
			}}
		}
		return list
	}
	return nil
}

// parent is where Esc goes from each list.
func (m *model) parent() screen {
	if m.screen == periodScreen {
		return filtersScreen
	}
	return menuScreen
}

func (m *model) goTo(sc screen) {
	m.screen = sc
	if sc == periodScreen { // start on the period that's active now
		for i, p := range periods {
			if p.label == m.s.filter.SinceLabel {
				m.cursor[sc] = i
			}
		}
	}
}

func (m *model) listKey(msg tea.KeyMsg) tea.Cmd {
	items := m.items()
	cur := min(m.cursor[m.screen], max(len(items)-1, 0)) // the list may have shrunk
	m.status = ""

	switch msg.String() {
	case "up", "k":
		cur = (cur - 1 + len(items)) % len(items) // wrap around at the top
	case "down", "j", "tab":
		cur = (cur + 1) % len(items)
	case "home", "g":
		cur = 0
	case "end", "G":
		cur = len(items) - 1
	case "enter", "right", "l", " ":
		m.cursor[m.screen] = cur
		if run := items[cur].run; run != nil {
			return run()
		}
		return nil
	case "esc", "left", "h", "backspace", "q":
		if m.screen == menuScreen {
			return tea.Quit
		}
		m.screen = m.parent()
		return nil
	}
	m.cursor[m.screen] = cur
	return nil
}

// ---- the exclude text box ----

func (m *model) inputKey(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = filtersScreen
	case tea.KeyEnter:
		patterns := strings.Fields(m.input)
		if len(patterns) > 0 {
			m.status = excludeStatus(patterns, m.s.addExcludes(patterns))
		}
		m.screen = filtersScreen
	case tea.KeyBackspace:
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.input += " "
	case tea.KeyRunes:
		m.input += string(msg.Runes)
	}
}

// ---- pages ----

// openPage runs a command and shows what it prints, scrollable.
func (m *model) openPage(title string, cmd func(*Session, []string) error, args []string) {
	var buf bytes.Buffer // capture the output instead of printing it
	out := m.s.out
	m.s.out = &buf
	if err := cmd(m.s, args); err != nil {
		m.s.fail(err)
	}
	m.s.out = out

	m.title = title
	m.lines = strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	m.offset = 0
	m.back = m.screen
	m.screen = pageScreen
}

func (m *model) pageKey(msg tea.KeyMsg) {
	page := m.bodyHeight()
	switch msg.String() {
	case "up", "k":
		m.scrollPage(-1)
	case "down", "j":
		m.scrollPage(1)
	case "pgup", "b":
		m.scrollPage(-page)
	case "pgdown", " ", "f":
		m.scrollPage(page)
	case "home", "g":
		m.offset = 0
	case "end", "G":
		m.scrollPage(len(m.lines))
	case "esc", "left", "h", "backspace", "q", "enter":
		m.screen = m.back
	}
}

func (m *model) scrollPage(by int) {
	maxOffset := max(len(m.lines)-m.bodyHeight(), 0)
	m.offset = min(max(m.offset+by, 0), maxOffset)
}

// ---- drawing ----

// View draws the whole screen: header, title, body, then the key hints.
func (m *model) View() string {
	var b strings.Builder
	b.WriteString(m.header())

	switch m.screen {
	case pageScreen:
		b.WriteString(" " + render.Bold(m.title) + "\n\n")
		end := min(m.offset+m.bodyHeight(), len(m.lines))
		for _, l := range m.lines[m.offset:end] {
			b.WriteString(l + "\n")
		}
		for range m.bodyHeight() - (end - m.offset) { // pad so the hints stay at the bottom
			b.WriteString("\n")
		}
		hint := "esc back"
		if len(m.lines) > m.bodyHeight() {
			hint = fmt.Sprintf("↑↓ scroll · space next page · esc back   %d–%d of %d", m.offset+1, end, len(m.lines))
		}
		b.WriteString("\n " + render.Dim(hint))

	case excludeScreen:
		b.WriteString(" " + render.Bold("Exclude files") + "\n\n")
		b.WriteString(" Patterns, separated by spaces:\n\n")
		b.WriteString(" > " + m.input + "█\n\n")
		b.WriteString(render.Dim(" e.g.  *.lock   package-lock.json   dist/   src/gen/*.go") + "\n\n")
		b.WriteString(" " + render.Dim("enter apply · esc cancel"))

	default:
		titles := map[screen]string{menuScreen: "", membersScreen: "Member", filtersScreen: "Filters", periodScreen: "Period"}
		if t := titles[m.screen]; t != "" {
			b.WriteString(" " + render.Bold(t) + "\n\n")
		}
		b.WriteString(m.list())
		if m.status != "" {
			b.WriteString("\n " + m.status + "\n")
		}
		hint := "↑↓ move · enter open · esc back · q quit"
		if m.screen == menuScreen {
			hint = "↑↓ move · enter open · q quit"
		}
		b.WriteString("\n " + render.Dim(hint))
	}
	return b.String()
}

// header is the same on every screen:
//
//	gh-snitch  a-narquel/proj · main · Sep 14 → Nov 6 (53 days)
//	filters: last 2 weeks · without *.lock
func (m *model) header() string {
	s := m.s
	parts := []string{render.Bold(s.repo)}
	if s.branch != "" {
		parts = append(parts, s.branch)
	}
	parts = append(parts, period(s.view()))
	h := "\n " + render.Tint(0, "gh-snitch") + "  " + strings.Join(parts, " · ") + "\n"
	if f := s.filterLabel(); f != "" {
		h += " " + render.Yellow("filters: "+f) + "\n"
	}
	if s.ghNote != "" {
		h += " " + render.Dim("("+s.ghNote+": PR and review numbers are off)") + "\n"
	}
	return h + "\n"
}

// list draws the current list with the cursor, scrolled so the cursor is visible.
func (m *model) list() string {
	items := m.items()
	cur := min(m.cursor[m.screen], max(len(items)-1, 0))

	labelW := 0
	for _, it := range items {
		labelW = max(labelW, render.Width(it.label))
	}

	visible := max(m.bodyHeight()-2, 3) // leave room for the status line
	start := 0
	if cur >= visible {
		start = cur - visible + 1
	}
	end := min(start+visible, len(items))

	var b strings.Builder
	for i := start; i < end; i++ {
		it := items[i]
		line := render.Pad(it.label, labelW)
		if it.desc != "" {
			line += "   " + render.Dim(it.desc)
		}
		if i == cur {
			b.WriteString(" " + render.Tint(0, "❯ ") + render.Bold(line) + "\n")
		} else {
			b.WriteString("   " + line + "\n")
		}
	}
	if end < len(items) {
		b.WriteString(render.Dim(fmt.Sprintf("   … %d more", len(items)-end)) + "\n")
	}
	return b.String()
}

// filterSummary is the dim text next to "Filters" in the main menu.
func (m *model) filterSummary() string {
	if f := m.s.filterLabel(); f != "" {
		return f
	}
	return "period, excluded files"
}

// bodyHeight is how many lines the page or list can use: the terminal height
// minus the header, the title and the hints.
func (m *model) bodyHeight() int {
	h := m.height
	if h == 0 {
		h = 24 // before the first resize event, assume a classic terminal
	}
	used := strings.Count(m.header(), "\n") + 2 /* title */ + 2 /* hints */
	return max(h-used, 3)
}
