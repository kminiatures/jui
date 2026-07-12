// Package ui implements the interactive task picker for jui.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"jui/internal/justfile"
)

type mode int

const (
	modeList mode = iota
	modeArgs
)

// Result is what the UI hands back to the caller after it exits.
type Result struct {
	Recipe *justfile.Recipe
	Args   []string
	Ran    bool
}

// Model is the Bubble Tea model driving the picker.
type Model struct {
	dump     *justfile.Dump
	all      []*justfile.Recipe
	filtered []*justfile.Recipe
	cursor   int
	scroll   int

	filter      textinput.Model
	showPrivate bool

	mode      mode
	argInputs []textinput.Model
	argFocus  int
	target    *justfile.Recipe

	width, height int

	Result Result
}

var (
	colorAccent    = lipgloss.Color("212")
	colorMuted     = lipgloss.Color("243")
	colorGroup     = lipgloss.Color("110")
	colorBorder    = lipgloss.Color("238")
	colorAlias     = lipgloss.Color("179")
	colorParam     = lipgloss.Color("150")
	styleTitle     = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleMuted     = lipgloss.NewStyle().Foreground(colorMuted)
	styleSelected  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(colorAccent)
	styleGroupTag  = lipgloss.NewStyle().Foreground(colorGroup)
	styleHeaderKey = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	styleBody      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleHelp      = lipgloss.NewStyle().Foreground(colorMuted)
	stylePane      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder)
)

// New builds the initial model for the given dump.
func New(dump *justfile.Dump) *Model {
	ti := textinput.New()
	ti.Placeholder = "type to filter…"
	ti.Prompt = "🔍 "
	ti.Focus()

	m := &Model{
		dump:   dump,
		all:    dump.SortedRecipes(false),
		filter: ti,
		mode:   modeList,
	}
	m.recompute()
	return m
}

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) recompute() {
	all := m.dump.SortedRecipes(m.showPrivate)
	m.all = all
	query := strings.TrimSpace(m.filter.Value())
	if query == "" {
		m.filtered = all
	} else {
		src := recipeSource(all)
		matches := fuzzy.FindFrom(query, src)
		out := make([]*justfile.Recipe, 0, len(matches))
		for _, mm := range matches {
			out = append(out, all[mm.Index])
		}
		m.filtered = out
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

type recipeSource []*justfile.Recipe

func (s recipeSource) String(i int) string {
	r := s[i]
	doc := ""
	if r.Doc != nil {
		doc = *r.Doc
	}
	return r.Name + " " + doc + " " + r.Group
}

func (s recipeSource) Len() int { return len(s) }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.mode == modeArgs {
			return m.updateArgs(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m *Model) current() *justfile.Recipe {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return m.filtered[m.cursor]
}

func (m *Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.Result = Result{}
		return m, tea.Quit
	case "esc":
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.recompute()
			return m, nil
		}
		m.Result = Result{}
		return m, tea.Quit
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "ctrl+n":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil
	case "ctrl+a":
		m.showPrivate = !m.showPrivate
		m.recompute()
		return m, nil
	case "enter", "tab":
		r := m.current()
		if r == nil {
			return m, nil
		}
		if len(r.Parameters) == 0 {
			m.Result = Result{Recipe: r, Ran: true}
			return m, tea.Quit
		}
		m.enterArgs(r)
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.recompute()
	return m, cmd
}

func (m *Model) enterArgs(r *justfile.Recipe) {
	m.target = r
	m.mode = modeArgs
	m.argFocus = 0
	m.argInputs = make([]textinput.Model, len(r.Parameters))
	for i, p := range r.Parameters {
		ti := textinput.New()
		ti.Prompt = p.Name + " = "
		if p.Default != "" {
			ti.Placeholder = p.Default
		} else if p.Variadic() {
			ti.Placeholder = "(space separated, optional)"
		} else {
			ti.Placeholder = "(required)"
		}
		if i == 0 {
			ti.Focus()
		}
		m.argInputs[i] = ti
	}
}

func (m *Model) updateArgs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.Result = Result{}
		return m, tea.Quit
	case "esc":
		m.mode = modeList
		return m, nil
	case "tab", "down":
		m.argInputs[m.argFocus].Blur()
		m.argFocus = (m.argFocus + 1) % len(m.argInputs)
		m.argInputs[m.argFocus].Focus()
		return m, nil
	case "shift+tab", "up":
		m.argInputs[m.argFocus].Blur()
		m.argFocus = (m.argFocus - 1 + len(m.argInputs)) % len(m.argInputs)
		m.argInputs[m.argFocus].Focus()
		return m, nil
	case "enter":
		args := make([]string, 0, len(m.argInputs))
		for i, ti := range m.argInputs {
			v := strings.TrimSpace(ti.Value())
			p := m.target.Parameters[i]
			if v == "" {
				if p.Variadic() {
					continue
				}
				if p.HasDefault {
					continue
				}
				args = append(args, "")
				continue
			}
			if p.Variadic() {
				args = append(args, strings.Fields(v)...)
			} else {
				args = append(args, v)
			}
		}
		m.Result = Result{Recipe: m.target, Args: args, Ran: true}
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.argInputs[m.argFocus], cmd = m.argInputs[m.argFocus].Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	header := m.viewHeader()
	help := m.viewHelp()
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(help) - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	if m.mode == modeArgs {
		return header + "\n" + m.viewArgs(bodyHeight) + "\n" + help
	}

	listWidth := m.width * 2 / 5
	if listWidth < 24 {
		listWidth = 24
	}
	previewWidth := m.width - listWidth - 3
	if previewWidth < 20 {
		previewWidth = 20
	}

	list := stylePane.Width(listWidth - 2).Height(bodyHeight - 2).Render(m.viewList(bodyHeight - 2))
	preview := stylePane.Width(previewWidth - 2).Height(bodyHeight - 2).Render(m.viewPreview(previewWidth-4, bodyHeight-2))
	row := lipgloss.JoinHorizontal(lipgloss.Top, list, preview)

	return header + "\n" + row + "\n" + help
}

func (m *Model) viewHeader() string {
	title := styleTitle.Render("jui")
	sub := styleMuted.Render(fmt.Sprintf(" — %s", m.dump.Source))
	filterLine := m.filter.View()
	return title + sub + "\n" + filterLine
}

func (m *Model) viewHelp() string {
	if m.mode == modeArgs {
		return styleHelp.Render("tab/↑↓ move · enter run · esc back · ctrl+c quit")
	}
	privacy := "off"
	if m.showPrivate {
		privacy = "on"
	}
	return styleHelp.Render(fmt.Sprintf("↑↓ move · enter run · ctrl+a private:%s · esc clear/quit · ctrl+c quit", privacy))
}

func (m *Model) viewList(height int) string {
	if len(m.filtered) == 0 {
		return styleMuted.Render("no matching recipes")
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+height {
		m.scroll = m.cursor - height + 1
	}
	end := m.scroll + height
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	var b strings.Builder
	for i := m.scroll; i < end; i++ {
		r := m.filtered[i]
		line := formatRecipeRow(r)
		if i == m.cursor {
			b.WriteString(styleSelected.Render(" " + line + " "))
		} else {
			b.WriteString(" " + line)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatRecipeRow(r *justfile.Recipe) string {
	name := r.Name
	if r.Private {
		name = styleMuted.Render(name)
	}
	tag := ""
	if r.Group != "" {
		tag = " " + styleGroupTag.Render("["+r.Group+"]")
	}
	return name + tag
}

func (m *Model) viewPreview(width, height int) string {
	r := m.current()
	if r == nil {
		return styleMuted.Render("no recipe selected")
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render(r.Name))
	if aliases := m.dump.AliasesFor(r.Name); len(aliases) > 0 {
		b.WriteString(styleMuted.Render(" (alias: " + strings.Join(aliases, ", ") + ")"))
	}
	b.WriteString("\n")
	if r.Group != "" {
		b.WriteString(styleGroupTag.Render("group: " + r.Group))
		b.WriteString("\n")
	}
	if r.Doc != nil && *r.Doc != "" {
		b.WriteString("\n")
		b.WriteString(styleBody.Render(*r.Doc))
		b.WriteString("\n")
	}
	if len(r.Parameters) > 0 {
		b.WriteString("\n")
		b.WriteString(styleHeaderKey.Render("parameters"))
		b.WriteString("\n")
		for _, p := range r.Parameters {
			b.WriteString("  " + lipgloss.NewStyle().Foreground(colorParam).Render(p.Signature()))
			b.WriteString("\n")
		}
	}
	if len(r.Dependencies) > 0 {
		b.WriteString("\n")
		b.WriteString(styleHeaderKey.Render("depends on"))
		b.WriteString("\n")
		names := make([]string, len(r.Dependencies))
		for i, d := range r.Dependencies {
			names[i] = d.Recipe
		}
		b.WriteString("  " + strings.Join(names, ", "))
		b.WriteString("\n")
	}
	if len(r.Attrs) > 0 {
		b.WriteString("\n")
		b.WriteString(styleHeaderKey.Render("attributes"))
		b.WriteString("\n")
		b.WriteString("  [" + strings.Join(r.Attrs, "] [") + "]")
		b.WriteString("\n")
	}
	if len(r.Lines) > 0 {
		b.WriteString("\n")
		b.WriteString(styleHeaderKey.Render("recipe"))
		b.WriteString("\n")
		for _, line := range r.Lines {
			b.WriteString(styleMuted.Render("  " + line))
			b.WriteString("\n")
		}
	}
	return lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render(strings.TrimRight(b.String(), "\n"))
}

func (m *Model) viewArgs(height int) string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("run "+m.target.Name) + styleMuted.Render(" — fill in arguments"))
	b.WriteString("\n\n")
	for i, ti := range m.argInputs {
		p := m.target.Parameters[i]
		marker := "  "
		if i == m.argFocus {
			marker = styleAccentBullet()
		}
		b.WriteString(marker + ti.View())
		if p.HasDefault {
			b.WriteString(styleMuted.Render("  (default: " + p.Default + ")"))
		}
		b.WriteString("\n")
	}
	return stylePane.Width(m.width - 2).Height(height - 2).Render(b.String())
}

func styleAccentBullet() string {
	return lipgloss.NewStyle().Foreground(colorAlias).Render("▸ ")
}
