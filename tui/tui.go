package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/yazdanctx/ggsnw/config"
	"github.com/yazdanctx/ggsnw/source"
	"github.com/yazdanctx/ggsnw/wordlist"
)

var Version = "dev"

type viewState int

const (
	viewModeSelect viewState = iota
	viewConfigPrompt
	viewMainMenu
	viewExpand
	viewExpandResult
	viewLoadFile
	viewExport
	viewWordlist
	viewClearConfirm
)

type model struct {
	state     viewState
	src       source.Source
	wl        *wordlist.Wordlist
	input     textinput.Model
	spinner   spinner.Model
	modeSel   int
	menuSel   int
	loading   bool

	cfgPromptFor string

	resultShortname string
	resultWords     []string
	resultErr       error
	resultIsFile    bool
	resultFileErrs  []string

	statusMsg string
}

func New() model {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 60
	ti.Placeholder = "e.g. auth"

	s := spinner.New()

	return model{
		state:   viewModeSelect,
		wl:      wordlist.New(),
		input:   ti,
		spinner: s,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

type (
	expandDoneMsg struct {
		shortname string
		words     []string
		err       error
	}
	fileExpandDoneMsg struct {
		words  []string
		errors []string
	}
	exportDoneMsg struct{ err error }
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if !m.loading {
			return m.handleKey(msg)
		}
		return m, nil
	}
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case expandDoneMsg:
		m.loading = false
		m.resultWords = msg.words
		m.resultErr = msg.err
		m.resultShortname = msg.shortname
		m.resultIsFile = false
		if msg.err == nil && len(msg.words) > 0 {
			m.wl.Add(msg.words...)
		}
		m.state = viewExpandResult
		return m, nil
	case fileExpandDoneMsg:
		m.loading = false
		if len(msg.words) > 0 {
			m.wl.Add(msg.words...)
		}
		m.resultWords = msg.words
		m.resultErr = nil
		m.resultFileErrs = msg.errors
		m.resultIsFile = true
		m.state = viewExpandResult
		return m, nil
	case exportDoneMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Export error: %v", msg.err)
		} else {
			m.statusMsg = "Wordlist exported!"
		}
		m.state = viewMainMenu
		return m, nil
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case viewModeSelect:
		return m.handleModeSelectKey(msg)
	case viewConfigPrompt:
		return m.handleConfigPromptKey(msg)
	case viewMainMenu:
		return m.handleMainMenuKey(msg)
	case viewExpand:
		return m.handleExpandKey(msg)
	case viewExpandResult:
		return m.handleExpandResultKey(msg)
	case viewLoadFile:
		return m.handleLoadFileKey(msg)
	case viewExport:
		return m.handleExportKey(msg)
	case viewWordlist:
		return m.handleWordlistKey(msg)
	case viewClearConfirm:
		return m.handleClearConfirmKey(msg)
	}
	return m, nil
}

func (m model) handleModeSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.modeSel > 0 {
			m.modeSel--
		}
	case "down", "j":
		if m.modeSel < 1 {
			m.modeSel++
		}
	case "enter":
		return m.selectMode()
	case "esc", "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) selectMode() (tea.Model, tea.Cmd) {
	cfg, cfgErr := config.Load()
	switch m.modeSel {
	case 0:
		if cfgErr == nil && cfg.GitHubToken != "" {
			m.src = source.NewGitHub(cfg.GitHubToken)
			m.state = viewMainMenu
			return m, nil
		}
		m.cfgPromptFor = "github"
		m.input.Placeholder = "Enter GitHub token (or blank to skip)"
		m.input.SetValue("")
		m.input.Focus()
		m.state = viewConfigPrompt
		return m, nil
	case 1:
		if cfgErr == nil && cfg.OpenAIKey != "" {
			m.src = source.NewOpenAI(cfg.OpenAIKey)
			m.state = viewMainMenu
			return m, nil
		}
		m.cfgPromptFor = "openai"
		m.input.Placeholder = "Enter OpenAI API key (or blank to skip)"
		m.input.SetValue("")
		m.input.Focus()
		m.state = viewConfigPrompt
		return m, nil
	}
	return m, nil
}

func (m model) handleConfigPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		m.input.Blur()
		if val == "" {
			m.state = viewModeSelect
			return m, nil
		}
		if m.cfgPromptFor == "github" {
			config.Save(func(c *config.Config) { c.GitHubToken = val })
			m.src = source.NewGitHub(val)
		} else {
			config.Save(func(c *config.Config) { c.OpenAIKey = val })
			m.src = source.NewOpenAI(val)
		}
		m.state = viewMainMenu
		return m, nil
	case "esc":
		m.input.Blur()
		m.state = viewModeSelect
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

var menuItems = []struct {
	label string
	action func(*model) (tea.Model, tea.Cmd)
}{
	{label: "Expand Shortname", action: func(m *model) (tea.Model, tea.Cmd) {
		m.input.SetValue("")
		m.input.Placeholder = "Enter shortname (e.g. auth)"
		m.input.Focus()
		m.state = viewExpand
		return m, nil
	}},
	{label: "Load from File", action: func(m *model) (tea.Model, tea.Cmd) {
		m.input.SetValue("")
		m.input.Placeholder = "Enter file path"
		m.input.Focus()
		m.state = viewLoadFile
		return m, nil
	}},
	{label: "View Wordlist", action: func(m *model) (tea.Model, tea.Cmd) {
		m.state = viewWordlist
		return m, nil
	}},
	{label: "Export Wordlist", action: func(m *model) (tea.Model, tea.Cmd) {
		m.input.SetValue("")
		m.input.Placeholder = "Enter output file path"
		m.input.Focus()
		m.state = viewExport
		return m, nil
	}},
	{label: "Clear Wordlist", action: func(m *model) (tea.Model, tea.Cmd) {
		m.state = viewClearConfirm
		return m, nil
	}},
	{label: "Quit", action: func(m *model) (tea.Model, tea.Cmd) {
		return m, tea.Quit
	}},
}

func (m model) handleMainMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuSel > 0 {
			m.menuSel--
		}
	case "down", "j":
		if m.menuSel < len(menuItems)-1 {
			m.menuSel++
		}
	case "enter":
		return menuItems[m.menuSel].action(&m)
	case "esc", "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) handleExpandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		shortname := strings.TrimSpace(m.input.Value())
		if shortname == "" {
			return m, nil
		}
		m.input.Blur()
		m.loading = true
		ctx := context.Background()
		return m, func() tea.Msg {
			words, err := m.src.Expand(ctx, shortname)
			return expandDoneMsg{shortname: shortname, words: words, err: err}
		}
	case "esc":
		m.input.Blur()
		m.state = viewMainMenu
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m model) handleExpandResultKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", " ":
		m.state = viewMainMenu
		return m, nil
	}
	return m, nil
}

func (m model) handleLoadFileKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		path := strings.TrimSpace(m.input.Value())
		if path == "" {
			return m, nil
		}
		m.input.Blur()
		m.loading = true
		ctx := context.Background()
		return m, func() tea.Msg {
			data, err := os.ReadFile(path)
			if err != nil {
				return fileExpandDoneMsg{errors: []string{fmt.Sprintf("Error reading file: %v", err)}}
			}
			var allWords []string
			var errors []string
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				words, err := m.src.Expand(ctx, line)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Error expanding %q: %v", line, err))
					continue
				}
				allWords = append(allWords, words...)
			}
			return fileExpandDoneMsg{words: allWords, errors: errors}
		}
	case "esc":
		m.input.Blur()
		m.state = viewMainMenu
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m model) handleExportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		path := strings.TrimSpace(m.input.Value())
		if path == "" {
			return m, nil
		}
		m.input.Blur()
		return m, func() tea.Msg {
			err := m.wl.Export(path)
			return exportDoneMsg{err: err}
		}
	case "esc":
		m.input.Blur()
		m.state = viewMainMenu
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m model) handleWordlistKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "b", "q":
		m.state = viewMainMenu
		return m, nil
	}
	return m, nil
}

func (m model) handleClearConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.wl.Clear()
		m.statusMsg = "Wordlist cleared"
		m.state = viewMainMenu
		return m, nil
	case "n", "N", "esc", "enter":
		m.state = viewMainMenu
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	switch m.state {
	case viewModeSelect:
		b.WriteString(fmt.Sprintf("ggsnw %s — Wordlist Generator\n\n", Version))
		b.WriteString("Select expansion source:\n\n")
		if m.modeSel == 0 {
			b.WriteString("  > GitHub (search code)\n")
			b.WriteString("    OpenAI (AI guesses)\n")
		} else {
			b.WriteString("    GitHub (search code)\n")
			b.WriteString("  > OpenAI (AI guesses)\n")
		}
		b.WriteString("\n\x1b[2m↑/↓ navigate  •  Enter select  •  Esc/q quit\x1b[0m\n")

	case viewConfigPrompt:
		if m.cfgPromptFor == "github" {
			b.WriteString("Configure GitHub Token\n\n")
			b.WriteString("Get one at: https://github.com/settings/tokens\n")
			b.WriteString("(no scopes needed for public repos)\n\n")
		} else {
			b.WriteString("Configure OpenAI API Key\n\n")
			b.WriteString("Get one at: https://platform.openai.com/api-keys\n\n")
		}
		b.WriteString(m.input.View() + "\n\n")
		b.WriteString("\x1b[2mEnter to save  •  Esc to go back\x1b[0m\n")

	case viewMainMenu:
		b.WriteString(fmt.Sprintf("ggsnw — Mode: %s    Words: %d\n\n", m.src.Name(), m.wl.Count()))
		for i, item := range menuItems {
			if m.menuSel == i {
				b.WriteString(fmt.Sprintf("  > %s\n", item.label))
			} else {
				b.WriteString(fmt.Sprintf("    %s\n", item.label))
			}
		}
		if m.statusMsg != "" {
			b.WriteString("\n  " + m.statusMsg + "\n")
			m.statusMsg = ""
		}
		b.WriteString("\n\x1b[2m↑/↓ navigate  •  Enter select  •  Esc/q quit\x1b[0m\n")

	case viewExpand:
		if m.loading {
			b.WriteString(m.spinner.View() + " Expanding...\n")
		} else {
			b.WriteString("Enter shortname:\n\n")
			b.WriteString(m.input.View() + "\n\n")
			b.WriteString("\x1b[2mEnter to expand  •  Esc to go back\x1b[0m\n")
		}

	case viewExpandResult:
		if m.resultIsFile {
			b.WriteString("File Load Results\n\n")
			if len(m.resultFileErrs) > 0 {
				for _, e := range m.resultFileErrs {
					b.WriteString("  ! " + e + "\n")
				}
				b.WriteString("\n")
			}
			if m.resultWords != nil {
				b.WriteString(fmt.Sprintf("  Added %d words\n", len(m.resultWords)))
			}
		} else if m.resultErr != nil {
			b.WriteString(fmt.Sprintf("Error expanding %q:\n  %v\n", m.resultShortname, m.resultErr))
		} else if len(m.resultWords) == 0 {
			b.WriteString(fmt.Sprintf("  [ ] %s — not found\n", m.resultShortname))
		} else {
			b.WriteString(fmt.Sprintf("  [x] %s — %d words\n\n", m.resultShortname, len(m.resultWords)))
			for _, w := range m.resultWords {
				b.WriteString("  " + w + "\n")
			}
		}
		b.WriteString(fmt.Sprintf("\nWordlist now has %d words\n", m.wl.Count()))
		b.WriteString("\n\x1b[2mPress Enter to continue\x1b[0m\n")

	case viewLoadFile:
		if m.loading {
			b.WriteString(m.spinner.View() + " Loading shortnames...\n")
		} else {
			b.WriteString("Load shortnames from file:\n\n")
			b.WriteString(m.input.View() + "\n\n")
			b.WriteString("\x1b[2mEnter to load  •  Esc to go back\x1b[0m\n")
		}

	case viewExport:
		b.WriteString("Export wordlist to file:\n\n")
		b.WriteString(m.input.View() + "\n\n")
		b.WriteString("\x1b[2mEnter to export  •  Esc to go back\x1b[0m\n")

	case viewWordlist:
		b.WriteString("Wordlist\n\n")
		b.WriteString(fmt.Sprintf("Total words: %d\n\n", m.wl.Count()))
		words := m.wl.All()
		if len(words) > 0 {
			for i, w := range words {
				b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, w))
			}
		} else {
			b.WriteString("  (empty)\n")
		}
		b.WriteString("\n\x1b[2mEsc to go back\x1b[0m\n")

	case viewClearConfirm:
		b.WriteString("Clear Wordlist\n\n")
		b.WriteString(fmt.Sprintf("Wordlist contains %d words.\nAre you sure? (y/N)\n", m.wl.Count()))
	}

	return b.String()
}
