package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lignumqt/envsnap/internal/types"
)

// ── styles (browser-local, read-only variant) ────────────────────────────────

var (
	styleBrwHeader     = lipgloss.NewStyle().Bold(true).Underline(true)
	styleBrwHint       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleBrwFilterLine = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleBrwCursor     = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

	styleBrwApt = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	styleBrwDnf = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// ── model ────────────────────────────────────────────────────────────────────

// pkgBrowserModel is a read-only, scrollable + filterable package viewer.
type pkgBrowserModel struct {
	packages  []types.Package
	cursor    int
	offset    int
	height    int
	filter    string
	filtering bool
	filtered  []int
	quit      bool
}

func newPkgBrowserModel(pkgs []types.Package) pkgBrowserModel {
	m := pkgBrowserModel{
		packages: pkgs,
		height:   24, // default; adjusted on WindowSizeMsg
	}
	m.rebuildFilter()
	return m
}

func (m pkgBrowserModel) Init() tea.Cmd { return nil }

func (m pkgBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		// Reserve rows for: header(1) + hints(1) + optional filter(1) + blank(1)
		// + column headers(1) + footer(1) = 6 overhead lines.
		h := msg.Height - 7
		if h < 5 {
			h = 5
		}
		m.height = h
		return m, nil

	case tea.KeyMsg:
		// ── filter mode ────────────────────────────────────────────
		if m.filtering {
			switch msg.String() {
			case "ctrl+c", "q", "esc":
				m.filtering = false
				if msg.String() == "ctrl+c" || msg.String() == "q" {
					m.quit = true
					return m, tea.Quit
				}
			case "enter":
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.rebuildFilter()
					m.cursor = 0
					m.offset = 0
				}
			default:
				if len(msg.Runes) > 0 {
					m.filter += string(msg.Runes)
					m.rebuildFilter()
					m.cursor = 0
					m.offset = 0
				}
			}
			return m, nil
		}

		// ── navigation mode ───────────────────────────────────────
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			if msg.String() == "esc" && m.filter != "" {
				// Esc with active filter: clear filter instead of quitting.
				m.filter = ""
				m.rebuildFilter()
				m.cursor = 0
				m.offset = 0
			} else {
				m.quit = true
				return m, tea.Quit
			}

		case "/":
			m.filtering = true

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}

		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.height {
					m.offset = m.cursor - m.height + 1
				}
			}

		case "pgup", "ctrl+u":
			m.cursor -= m.height
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.offset -= m.height
			if m.offset < 0 {
				m.offset = 0
			}

		case "pgdown", "ctrl+d":
			m.cursor += m.height
			if m.cursor >= len(m.filtered) {
				m.cursor = len(m.filtered) - 1
			}
			m.offset += m.height
			maxOffset := len(m.filtered) - m.height
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.offset > maxOffset {
				m.offset = maxOffset
			}

		case "g", "home":
			m.cursor = 0
			m.offset = 0

		case "G", "end":
			m.cursor = len(m.filtered) - 1
			m.offset = m.cursor - m.height + 1
			if m.offset < 0 {
				m.offset = 0
			}
		}
	}
	return m, nil
}

func (m *pkgBrowserModel) rebuildFilter() {
	m.filtered = nil
	f := strings.ToLower(m.filter)
	for i, pkg := range m.packages {
		if f != "" && !strings.Contains(strings.ToLower(pkg.Name), f) &&
			!strings.Contains(strings.ToLower(pkg.Version), f) {
			continue
		}
		m.filtered = append(m.filtered, i)
	}
}

func (m pkgBrowserModel) View() string {
	if m.quit {
		return ""
	}

	var sb strings.Builder

	// ── header ────────────────────────────────────────────────────
	title := fmt.Sprintf("Packages — %d total", len(m.packages))
	if m.filter != "" {
		title += fmt.Sprintf("  (filter: %d shown)", len(m.filtered))
	}
	sb.WriteString(styleBrwHeader.Render(title) + "\n")

	// ── hint bar ──────────────────────────────────────────────────
	if m.filtering {
		sb.WriteString(styleBrwHint.Render("  FILTER MODE: type to search  Enter accept  Esc back") + "\n")
	} else {
		sb.WriteString(styleBrwHint.Render("  ↑/↓/j/k  PgUp/PgDn  g/G home/end  / filter  Esc clear filter  q quit") + "\n")
	}

	// ── optional filter line ──────────────────────────────────────
	if m.filter != "" || m.filtering {
		cursor := ""
		if m.filtering {
			cursor = "█"
		}
		sb.WriteString(styleBrwFilterLine.Render("  Filter: "+m.filter+cursor) + "\n")
	}

	sb.WriteString("\n")

	// ── column headers ────────────────────────────────────────────
	sb.WriteString(styleBrwHint.Render(fmt.Sprintf("  %-40s  %-22s  %s", "NAME", "VERSION", "MGR")) + "\n")

	// ── rows ──────────────────────────────────────────────────────
	if len(m.filtered) == 0 {
		sb.WriteString("  No packages match filter.\n")
	} else {
		end := m.offset + m.height
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := m.offset; i < end; i++ {
			idx := m.filtered[i]
			pkg := m.packages[idx]

			var mgrBadge string
			switch pkg.Manager {
			case "apt":
				mgrBadge = styleBrwApt.Render("[apt]")
			case "dnf":
				mgrBadge = styleBrwDnf.Render("[dnf]")
			default:
				mgrBadge = "[" + pkg.Manager + "]"
			}

			name := pkg.Name
			if len(name) > 40 {
				name = name[:39] + "…"
			}
			ver := pkg.Version
			if len(ver) > 22 {
				ver = ver[:21] + "…"
			}

			line := fmt.Sprintf("  %-40s  %-22s  %s", name, ver, mgrBadge)
			if i == m.cursor {
				sb.WriteString(styleBrwCursor.Render(line) + "\n")
			} else {
				sb.WriteString(line + "\n")
			}
		}
	}

	// ── footer ────────────────────────────────────────────────────
	if len(m.filtered) > 0 {
		from := m.offset + 1
		to := m.offset + m.height
		if to > len(m.filtered) {
			to = len(m.filtered)
		}
		sb.WriteString(styleBrwHint.Render(fmt.Sprintf("  %d–%d of %d", from, to, len(m.filtered))) + "\n")
	}

	return sb.String()
}

// runPackagesBrowser opens the full-screen read-only TUI browser.
// Returns after the user presses q / Esc / Ctrl+C.
func runPackagesBrowser(pkgs []types.Package) error {
	m := newPkgBrowserModel(pkgs)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
