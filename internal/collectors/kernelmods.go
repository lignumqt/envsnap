package collectors

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lignumqt/envsnap/internal/types"
)

const KernelModsCollectorName = "kernel_modules"

// KernelModsCollector reads loaded kernel modules via /proc/modules and lets
// the user interactively select which ones to include in the snapshot.
// When stdin is not a terminal (e.g. piped) all modules are included automatically.
type KernelModsCollector struct{}

func NewKernelModsCollector() *KernelModsCollector { return &KernelModsCollector{} }

func (c *KernelModsCollector) Name() string { return KernelModsCollectorName }

func (c *KernelModsCollector) Collect(ctx context.Context) (Section, error) {
	mods, err := collectAllModules(ctx)
	if err != nil {
		return Section{}, fmt.Errorf("kernel modules: %w", err)
	}
	if len(mods) == 0 {
		return Section{Name: KernelModsCollectorName, Data: []types.KernelModule{}}, nil
	}

	// Non-interactive fallback: save only loaded modules to keep snapshot compact.
	if !isTTY() || IsNonInteractive(ctx) {
		return Section{Name: KernelModsCollectorName, Data: loadedOnly(mods)}, nil
	}

	selected, err := runTUI(mods)
	if err != nil {
		// TUI failure — fall back to loaded-only.
		return Section{Name: KernelModsCollectorName, Data: loadedOnly(mods)}, nil
	}
	return Section{Name: KernelModsCollectorName, Data: selected}, nil
}

func loadedOnly(mods []types.KernelModule) []types.KernelModule {
	var out []types.KernelModule
	for _, m := range mods {
		if m.Loaded {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return mods // nothing loaded? return everything
	}
	return out
}

// ── lsmod parsing ─────────────────────────────────────────────────────────────

func parseLsmod(ctx context.Context) ([]types.KernelModule, error) {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		// Fallback to lsmod command.
		return parseLsmodCmd(ctx)
	}
	return parseProcModules(data), nil
}

// parseProcModules parses /proc/modules.
// Each line: name size usecount usedby state offset
func parseProcModules(data []byte) []types.KernelModule {
	var mods []types.KernelModule
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		size, _ := strconv.ParseInt(parts[1], 10, 64)
		usedBy := strings.Trim(parts[3], ",")
		if usedBy == "-" {
			usedBy = ""
		}
		mods = append(mods, types.KernelModule{
			Name:   parts[0],
			Size:   size,
			UsedBy: usedBy,
			Loaded: true,
		})
	}
	return mods
}

func parseLsmodCmd(ctx context.Context) ([]types.KernelModule, error) {
	out, err := runCommand(ctx, "lsmod")
	if err != nil {
		return nil, err
	}
	var mods []types.KernelModule
	scanner := bufio.NewScanner(strings.NewReader(out))
	first := true
	for scanner.Scan() {
		if first { // skip header line
			first = false
			continue
		}
		parts := strings.Fields(scanner.Text())
		if len(parts) < 3 {
			continue
		}
		size, _ := strconv.ParseInt(parts[1], 10, 64)
		usedBy := ""
		if len(parts) > 3 {
			usedBy = strings.Join(parts[3:], ",")
		}
		mods = append(mods, types.KernelModule{
			Name:   parts[0],
			Size:   size,
			UsedBy: usedBy,
			Loaded: true,
		})
	}
	return mods, nil
}

// ── Backend: merge loaded + installed ─────────────────────────────────────────

// collectAllModules merges loaded modules (/proc/modules) and installed modules
// (/lib/modules/<kver>/modules.dep) into a single de-duplicated sorted list.
func collectAllModules(ctx context.Context) ([]types.KernelModule, error) {
	loaded, err := parseLsmod(ctx)
	if err != nil {
		debugf(ctx, "loaded modules error: %v", err)
	}
	loadedMap := make(map[string]*types.KernelModule, len(loaded))
	for i := range loaded {
		loadedMap[loaded[i].Name] = &loaded[i]
	}

	installedSet, depsMap, err := parseInstalledModules(ctx)
	if err != nil {
		debugf(ctx, "installed modules error (non-fatal): %v", err)
	}

	result := make(map[string]types.KernelModule, len(loadedMap)+len(installedSet))
	for name, m := range loadedMap {
		mod := *m
		if _, ok := installedSet[name]; ok {
			mod.Installed = true
		}
		if deps, ok := depsMap[name]; ok {
			mod.Depends = deps
		}
		result[name] = mod
	}
	for name := range installedSet {
		if _, ok := result[name]; ok {
			continue
		}
		mod := types.KernelModule{Name: name, Installed: true}
		if deps, ok := depsMap[name]; ok {
			mod.Depends = deps
		}
		result[name] = mod
	}

	all := make([]types.KernelModule, 0, len(result))
	for _, m := range result {
		all = append(all, m)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	debugf(ctx, "kernel modules: %d total (%d loaded, %d installed)", len(all), len(loadedMap), len(installedSet))
	return all, nil
}

func kernelRelease() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func moduleNameFromPath(path string) string {
	base := filepath.Base(path)
	for _, ext := range []string{".ko.xz", ".ko.gz", ".ko.zst", ".ko"} {
		if strings.HasSuffix(base, ext) {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	return strings.ReplaceAll(base, "-", "_")
}

// parseInstalledModules discovers all kernel modules under /lib/modules/<kver>/.
// It combines two sources:
//   - modules.dep  — for static dependency data
//   - directory walk — finds ALL .ko* files including DKMS / out-of-tree modules
//     that may not be listed in modules.dep (e.g. depmod has not been re-run yet)
func parseInstalledModules(ctx context.Context) (installed map[string]struct{}, deps map[string][]string, err error) {
	kver := kernelRelease()
	if kver == "" {
		return nil, nil, fmt.Errorf("cannot determine kernel version")
	}
	modDir := "/lib/modules/" + kver

	installed = make(map[string]struct{})
	deps = make(map[string][]string)

	// --- 1. Parse modules.dep for dependency data ---
	depFile := modDir + "/modules.dep"
	debugf(ctx, "reading %s", depFile)
	if data, readErr := os.ReadFile(depFile); readErr == nil {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			colonIdx := strings.IndexByte(line, ':')
			if colonIdx < 0 {
				continue
			}
			modName := moduleNameFromPath(strings.TrimSpace(line[:colonIdx]))
			installed[modName] = struct{}{}
			if rest := strings.TrimSpace(line[colonIdx+1:]); rest != "" {
				depPaths := strings.Fields(rest)
				depNames := make([]string, 0, len(depPaths))
				for _, dp := range depPaths {
					depNames = append(depNames, moduleNameFromPath(dp))
				}
				deps[modName] = depNames
			}
		}
	} else {
		debugf(ctx, "modules.dep not readable: %v", readErr)
	}

	// --- 2. Walk /lib/modules/<kver>/ to find every .ko* file ---
	// This catches DKMS / out-of-tree modules not yet registered in modules.dep.
	debugf(ctx, "walking %s for .ko* files", modDir)
	walkErr := filepath.WalkDir(modDir, func(path string, d os.DirEntry, e error) error {
		if e != nil || d.IsDir() {
			return nil
		}
		if isKoFile(d.Name()) {
			installed[moduleNameFromPath(d.Name())] = struct{}{}
		}
		return nil
	})
	if walkErr != nil {
		debugf(ctx, "walk %s error: %v", modDir, walkErr)
	}

	debugf(ctx, "installed modules found: %d", len(installed))
	return installed, deps, nil
}

// isKoFile reports whether a filename looks like a kernel module.
func isKoFile(name string) bool {
	for _, ext := range []string{".ko", ".ko.xz", ".ko.gz", ".ko.zst", ".ko.lz4"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// ── TTY detection ─────────────────────────────────────────────────────────────

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ── Bubbletea TUI ─────────────────────────────────────────────────────────────

const visibleRows = 20

var (
	styleSelected   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleCursor     = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	styleHeader     = lipgloss.NewStyle().Bold(true).Underline(true)
	styleHint       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleFilterLine = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

// tuiModel is the bubbletea model for the multi-select kernel module picker.
type tuiModel struct {
	modules     []types.KernelModule
	cursor      int
	selected    map[int]bool
	quitting    bool
	done        bool
	filter      string
	filtering   bool  // true = user is typing a filter query
	filtered    []int // indices into modules that match filter
	height      int
	offset      int  // scroll offset
	showAll     bool // false = loaded only, true = loaded + installed
	confirming  bool // true = confirmation dialog visible
	confirmSave bool // true = confirming save (Enter), false = confirming cancel (Ctrl+C)
}

func newTUIModel(mods []types.KernelModule) tuiModel {
	m := tuiModel{
		modules:  mods,
		selected: make(map[int]bool),
		showAll:  true, // default: show loaded + installed
		height:   visibleRows,
	}
	m.rebuildFilter()
	return m
}

func (m tuiModel) Init() tea.Cmd { return nil }

// nameIndex builds a name→slice-index lookup for fast dependency resolution.
func nameIndex(mods []types.KernelModule) map[string]int {
	idx := make(map[string]int, len(mods))
	for i, mod := range mods {
		idx[mod.Name] = i
	}
	return idx
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				// Exit filter mode; filter string kept.
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.rebuildFilter()
					m.cursor = 0
					m.offset = 0
				}
			default:
				if len(msg.String()) == 1 {
					m.filter += msg.String()
					m.rebuildFilter()
					m.cursor = 0
					m.offset = 0
				}
			}
			return m, nil
		}

		// ── Navigation mode ────────────────────────────────────────
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
			// Clear filter when Esc pressed outside filter mode.
			if m.filter != "" {
				m.filter = ""
				m.rebuildFilter()
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
				nowSelected := !m.selected[idx]
				m.selected[idx] = nowSelected
				// When selecting a module, auto-select all its dependencies.
				// Deselecting does NOT cascade — dependencies stay as-is.
				if nowSelected && len(m.modules[idx].Depends) > 0 {
					ni := nameIndex(m.modules)
					m.selectDeps(idx, ni, make(map[int]bool))
				}
			}

		case "a":
			// Toggle all visible modules.
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

		case "i":
			// Toggle between loaded-only view and all (loaded + installed) view.
			m.showAll = !m.showAll
			m.rebuildFilter()
			m.cursor = 0
			m.offset = 0
		}
	}
	return m, nil
}

// selectDeps recursively marks all dependencies of modules[idx] as selected.
// visited prevents infinite loops in case of circular dependency declarations.
func (m *tuiModel) selectDeps(idx int, ni map[string]int, visited map[int]bool) {
	if visited[idx] {
		return
	}
	visited[idx] = true
	for _, depName := range m.modules[idx].Depends {
		depIdx, ok := ni[depName]
		if !ok {
			continue
		}
		m.selected[depIdx] = true
		m.selectDeps(depIdx, ni, visited)
	}
}

func (m *tuiModel) rebuildFilter() {
	m.filtered = nil
	f := strings.ToLower(m.filter)
	for i, mod := range m.modules {
		if !m.showAll && !mod.Loaded {
			continue // hide installed-only modules in loaded-only view
		}
		if f != "" && !strings.Contains(strings.ToLower(mod.Name), f) {
			continue
		}
		m.filtered = append(m.filtered, i)
	}
}

func (m tuiModel) View() string {
	if m.quitting || m.done {
		return ""
	}

	// ── Confirmation overlay ─────────────────────────────────────────
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
			title = "✔ Save selection?"
			detail = fmt.Sprintf("%d module(s) selected", countSelected(m.selected))
		} else {
			title = "✖ Cancel without saving?"
			detail = "Selected modules will NOT be included in the snapshot"
		}
		content := title + "\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(detail) + "\n\n" +
			yesStyle.Render("[Y] Yes / Enter") + "   " +
			noStyle.Render("[N] No / Esc")
		sb.WriteString("\n\n" + boxStyle.Render(content) + "\n")
		return sb.String()
	}

	var sb strings.Builder

	var modeLabel string
	if m.showAll {
		modeLabel = "loaded + installed  (i = loaded only)"
	} else {
		modeLabel = "loaded only  (i = show all installed)"
	}
	sb.WriteString(styleHeader.Render("Select Kernel Modules — "+modeLabel) + "\n")
	if m.filtering {
		sb.WriteString(styleHint.Render("  FILTER MODE: type to search  Esc = back  Enter confirm  Ctrl+C cancel") + "\n")
	} else {
		sb.WriteString(styleHint.Render("  ↑/↓/j/k nav  SPACE toggle  a all  i toggle view  / filter  Enter confirm  Ctrl+C cancel") + "\n")
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
		sb.WriteString("  No modules match filter.\n")
	}

	end := m.offset + m.height
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.offset; i < end; i++ {
		idx := m.filtered[i]
		mod := m.modules[idx]

		checkbox := "[ ]"
		if m.selected[idx] {
			checkbox = "[x]"
		}

		badge := modBadge(mod)
		depsStr := modDepsStr(mod)
		var sizeStr string
		if mod.Size > 0 {
			sizeStr = fmt.Sprintf("%8d B", mod.Size)
		} else {
			sizeStr = "         -"
		}
		line := "  " + checkbox + " " +
			fmt.Sprintf("%-34s", mod.Name) + "  " +
			badge + "  " +
			sizeStr + "  " +
			depsStr

		if i == m.cursor {
			line = styleCursor.Render(line)
		} else if m.selected[idx] {
			line = styleSelected.Render(line)
		}
		sb.WriteString(line + "\n")
	}

	if len(m.filtered) > m.height {
		sb.WriteString(styleHint.Render(fmt.Sprintf(
			"\n  Showing %d–%d of %d | %d selected | [L]=loaded  [I]=installed",
			m.offset+1, end, len(m.filtered), countSelected(m.selected),
		)) + "\n")
	} else {
		sb.WriteString(styleHint.Render(fmt.Sprintf(
			"\n  %d visible | %d selected | [L]=loaded  [I]=installed",
			len(m.filtered), countSelected(m.selected),
		)) + "\n")
	}

	return sb.String()
}

// modBadge returns a fixed-width (6-char visible) colored badge for the module status.
func modBadge(m types.KernelModule) string {
	loadedSt := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	instSt := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	switch {
	case m.Loaded && m.Installed:
		return loadedSt.Render("[L]") + instSt.Render("[I]")
	case m.Loaded:
		return loadedSt.Render("[L]") + "   "
	case m.Installed:
		return "   " + instSt.Render("[I]")
	default:
		return "      "
	}
}

// modDepsStr returns a short textual summary of the module's static dependencies.
func modDepsStr(m types.KernelModule) string {
	if len(m.Depends) == 0 {
		return ""
	}
	if len(m.Depends) <= 3 {
		return "→ " + strings.Join(m.Depends, ", ")
	}
	return fmt.Sprintf("→ %s, … (+%d)", strings.Join(m.Depends[:3], ", "), len(m.Depends)-3)
}

func countSelected(sel map[int]bool) int {
	n := 0
	for _, v := range sel {
		if v {
			n++
		}
	}
	return n
}

func runTUI(mods []types.KernelModule) ([]types.KernelModule, error) {
	model := newTUIModel(mods)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m, ok := finalModel.(tuiModel)
	if !ok || m.quitting {
		return nil, fmt.Errorf("selection cancelled")
	}

	var selected []types.KernelModule
	for idx, isSelected := range m.selected {
		if isSelected {
			selected = append(selected, mods[idx])
		}
	}
	return selected, nil
}

// runCommand runs an external command and returns its stdout as a string.
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd := newExecCmd(ctx, name, args...)
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
