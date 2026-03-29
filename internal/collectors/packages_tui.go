package collectors

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lignumqt/envsnap/internal/types"
)

var (
	stylePkgApt = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	stylePkgDnf = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// pkgTUIModel is the bubbletea model for the multi-select package picker.
// All packages are selected by default; the user deselects what they don't want.
type pkgTUIModel struct {
	packages    []types.Package
	cursor      int
	selected    map[int]bool
	filter      string
	filtering   bool
	filtered    []int
	height      int
	offset      int
	confirming  bool
	confirmSave bool
	quitting    bool
	done        bool
}

func newPkgTUIModel(pkgs []types.Package) pkgTUIModel {
	sel := make(map[int]bool, len(pkgs))
	for i := range pkgs {
		sel[i] = true // all selected by default
	}
	m := pkgTUIModel{
		packages: pkgs,
		selected: sel,
		height:   visibleRows,
	}
	m.rebuildPkgFilter()
	return m
}

func (m pkgTUIModel) Init() tea.Cmd { return nil }

func (m pkgTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// ── Confirmation dialog ──────────────────────────────────
		if m.confirming {
			switch msg.String() {
			case "y", "enter":
				if m.confirmSave {
					m.done = true
				} else {
					m.quitting = true
				}
				return m, tea.Quit
			case "n", "esc":
				m.confirming = false
			}
			return m, nil
		}

		// ── Filter mode ────────────────────────────────────────────
		if m.filtering {
			switch msg.String() {
			case "ctrl+c":
				m.filtering = false
				m.confirming = true
				m.confirmSave = false
			case "enter":
				m.filtering = false
				m.confirming = true
				m.confirmSave = true
			case "esc", "/":
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.rebuildPkgFilter()
					m.cursor = 0
					m.offset = 0
				}
			default:
				if len(msg.String()) == 1 {
					m.filter += msg.String()
					m.rebuildPkgFilter()
					m.cursor = 0
					m.offset = 0
				}
			}
			return m, nil
		}

		// ── Navigation mode ───────────────────────────────────────
		switch msg.String() {
		case "ctrl+c":
			m.confirming = true
			m.confirmSave = false

		case "enter":
			m.confirming = true
			m.confirmSave = true

		case "/":
			m.filtering = true

		case "esc":
			if m.filter != "" {
				m.filter = ""
				m.rebuildPkgFilter()
				m.cursor = 0
				m.offset = 0
			}

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

		case " ":
			if len(m.filtered) > 0 {
				idx := m.filtered[m.cursor]
				m.selected[idx] = !m.selected[idx]
			}

		case "a":
			// Toggle all currently visible (filtered) packages.
			allSelected := true
			for _, idx := range m.filtered {
				if !m.selected[idx] {
					allSelected = false
					break
				}
			}
			for _, idx := range m.filtered {
				m.selected[idx] = !allSelected
			}
		}
	}
	return m, nil
}

func (m *pkgTUIModel) rebuildPkgFilter() {
	m.filtered = nil
	f := strings.ToLower(m.filter)
	for i, pkg := range m.packages {
		if f != "" && !strings.Contains(strings.ToLower(pkg.Name), f) {
			continue
		}
		m.filtered = append(m.filtered, i)
	}
}

func (m pkgTUIModel) View() string {
	if m.quitting || m.done {
		return ""
	}

	// ── Confirmation overlay ──────────────────────────────────────────
	if m.confirming {
		var sb strings.Builder
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 4).
			BorderForeground(lipgloss.Color("11"))
		yesStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
		noStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
		var title, detail string
		if m.confirmSave {
			n := countSelected(m.selected)
			title = "✔ Save selection?"
			detail = fmt.Sprintf("%d / %d package(s) will be included in snapshot", n, len(m.packages))
		} else {
			title = "✖ Cancel without saving?"
			detail = "All packages will be included in the snapshot (no filtering)"
		}
		content := title + "\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(detail) + "\n\n" +
			yesStyle.Render("[Y] Yes / Enter") + "   " +
			noStyle.Render("[N] No / Esc")
		sb.WriteString("\n\n" + boxStyle.Render(content) + "\n")
		return sb.String()
	}

	var sb strings.Builder
	nSelected := countSelected(m.selected)

	sb.WriteString(styleHeader.Render(
		fmt.Sprintf("Select Packages — %d total, %d selected", len(m.packages), nSelected),
	) + "\n")

	if m.filtering {
		sb.WriteString(styleHint.Render("  FILTER MODE: type to search  Esc = back  Enter confirm  Ctrl+C cancel") + "\n")
	} else {
		sb.WriteString(styleHint.Render("  ↑/↓/j/k nav  SPACE toggle  a all/none  / filter  Enter confirm  Ctrl+C cancel") + "\n")
	}

	if m.filter != "" {
		cursor := ""
		if m.filtering {
			cursor = "█"
		}
		sb.WriteString(styleFilterLine.Render("  Filter: "+m.filter+cursor) + "\n")
	} else if m.filtering {
		sb.WriteString(styleFilterLine.Render("  Filter: █") + "\n")
	}
	sb.WriteString("\n")

	if len(m.filtered) == 0 {
		sb.WriteString("  No packages match filter.\n")
	}

	end := m.offset + m.height
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.offset; i < end; i++ {
		idx := m.filtered[i]
		pkg := m.packages[idx]

		checkbox := "[ ]"
		if m.selected[idx] {
			checkbox = "[x]"
		}

		var mgrBadge string
		switch pkg.Manager {
		case "apt":
			mgrBadge = stylePkgApt.Render("[apt]")
		case "dnf":
			mgrBadge = stylePkgDnf.Render("[dnf]")
		default:
			mgrBadge = pkg.Manager
		}

		ver := pkg.Version
		if len(ver) > 32 {
			ver = ver[:29] + "…"
		}

		line := "  " + checkbox + " " +
			fmt.Sprintf("%-40s", pkg.Name) + "  " +
			fmt.Sprintf("%-33s", ver) + " " +
			mgrBadge

		if i == m.cursor {
			line = styleCursor.Render(line)
		} else if m.selected[idx] {
			line = styleSelected.Render(line)
		}
		sb.WriteString(line + "\n")
	}

	if len(m.filtered) > m.height {
		sb.WriteString(styleHint.Render(fmt.Sprintf(
			"\n  Showing %d–%d of %d | %d selected",
			m.offset+1, end, len(m.filtered), nSelected,
		)) + "\n")
	} else {
		sb.WriteString(styleHint.Render(fmt.Sprintf(
			"\n  %d visible | %d selected",
			len(m.filtered), nSelected,
		)) + "\n")
	}

	return sb.String()
}

// runPackagesTUI opens the interactive package selector and returns the packages the
// user kept selected.  If the user cancels (Ctrl+C → confirms cancel) the full
// original list is returned unchanged, so the snapshot is never silently empty.
func runPackagesTUI(pkgs []types.Package) ([]types.Package, error) {
	model := newPkgTUIModel(pkgs)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m, ok := finalModel.(pkgTUIModel)
	if !ok || m.quitting {
		// User cancelled — return all packages unchanged.
		return pkgs, nil
	}

	selected := make([]types.Package, 0, len(pkgs))
	for idx := range pkgs {
		if m.selected[idx] {
			selected = append(selected, pkgs[idx])
		}
	}
	return selected, nil
}
