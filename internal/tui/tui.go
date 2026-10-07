// Package tui is the interactive account picker.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
	"github.com/uzairrehman4/gitswitch/internal/ops"
	"github.com/uzairrehman4/gitswitch/internal/profile"
)

var (
	accent  = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A78BFA"}
	green   = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	red     = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	dim     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	title   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	subtle  = lipgloss.NewStyle().Foreground(dim)
	card    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dim).Padding(0, 2).Width(64)
	cardSel = card.BorderForeground(accent)
)

type mode int

const (
	modeList mode = iota
	modeForm
	modeDelete
)

type field int

const (
	fLabel field = iota
	fGitName
	fEmail
	fUser
	fHost
	fKey
)

var fieldMeta = map[field]struct{ label, placeholder string }{
	fLabel:   {"Label", "work"},
	fGitName: {"Git name", "Ada Lovelace"},
	fEmail:   {"Git email", "ada@example.com"},
	fUser:    {"Username", "optional, e.g. ada-dev"},
	fHost:    {"Host", "github.com"},
	fKey:     {"SSH key", "path to private key, blank for none"},
}

type testDoneMsg struct {
	name, user string
	err        error
}

type model struct {
	store  *profile.Store
	cursor int
	cwd    string
	inRepo bool
	links  []gitx.Link
	global gitx.Identity // cached; git is only re-queried after a change
	here   gitx.Identity
	msg    string
	msgErr bool
	busy   string // profile name being tested
	pub    string // public key to show after creating a profile

	mode    mode
	fields  []field
	inputs  []textinput.Model
	focus   int
	editing string // profile being edited; "" when adding
	formErr string
}

// Run starts the picker on the real terminal.
func Run(store *profile.Store) error { return RunWith(store) }

// RunWith starts the picker with custom program options (used by tests).
func RunWith(store *profile.Store, opts ...tea.ProgramOption) error {
	cwd, _ := os.Getwd()
	m := model{store: store, cwd: cwd, inRepo: gitx.InRepo()}
	m.reload()
	for i, p := range store.Profiles {
		if strings.EqualFold(p.Email, m.global.Email) {
			m.cursor = i
		}
	}
	_, err := tea.NewProgram(m, opts...).Run()
	return err
}

func (m *model) reload() {
	m.links = gitx.Links(m.store.ProfilesDir())
	m.global = gitx.GlobalIdentity()
	m.here = gitx.Effective()
}

func (m model) Init() tea.Cmd { return nil }

func (m model) selected() (profile.Profile, bool) {
	if len(m.store.Profiles) == 0 || m.cursor >= len(m.store.Profiles) {
		return profile.Profile{}, false
	}
	return m.store.Profiles[m.cursor], true
}

func (m *model) say(s string, isErr bool) { m.msg, m.msgErr = s, isErr }

// openForm prepares the add (p == nil) or edit form.
func (m *model) openForm(p *profile.Profile) tea.Cmd {
	m.mode, m.formErr, m.focus, m.pub = modeForm, "", 0, ""
	values := map[field]string{}
	if p == nil {
		m.editing = ""
		m.fields = []field{fLabel, fGitName, fEmail, fUser, fHost}
		if _, known := m.store.MatchIdentity(m.global.Name, m.global.Email); !known {
			values[fGitName], values[fEmail] = m.global.Name, m.global.Email
		}
	} else {
		m.editing = p.Name
		m.fields = []field{fGitName, fEmail, fUser, fHost, fKey}
		values = map[field]string{fGitName: p.GitName, fEmail: p.Email, fUser: p.GitHub, fHost: p.Host, fKey: p.KeyPath}
	}
	m.inputs = make([]textinput.Model, len(m.fields))
	for i, f := range m.fields {
		t := textinput.New()
		t.Prompt = ""
		t.Placeholder = fieldMeta[f].placeholder
		t.SetValue(values[f])
		t.CharLimit = 256
		t.Width = 44
		m.inputs[i] = t
	}
	m.inputs[0].Focus()
	return textinput.Blink
}

func (m *model) moveFocus(delta int) {
	m.inputs[m.focus].Blur()
	m.focus = (m.focus + delta + len(m.inputs)) % len(m.inputs)
	m.inputs[m.focus].Focus()
}

func (m *model) submitForm() {
	vals := map[field]string{}
	for i, f := range m.fields {
		vals[f] = strings.TrimSpace(m.inputs[i].Value())
	}
	if m.editing == "" {
		p := profile.Profile{Name: vals[fLabel], GitName: vals[fGitName], Email: vals[fEmail], GitHub: vals[fUser], Host: vals[fHost]}
		res, err := ops.Add(m.store, p, true)
		if err != nil {
			m.formErr = err.Error()
			return
		}
		m.mode = modeList
		m.cursor = indexOf(m.store, res.Profile.Name)
		m.say(fmt.Sprintf("Added %q", res.Profile.Name), false)
		if res.PublicKey != "" {
			m.pub = res.PublicKey
			if gitx.CopyToClipboard(res.PublicKey) {
				m.pub += "\n(copied to clipboard)"
			}
		}
		return
	}
	old, _ := m.store.Get(m.editing)
	p := profile.Profile{Name: old.Name, GitName: vals[fGitName], Email: vals[fEmail], GitHub: vals[fUser], Host: vals[fHost], KeyPath: vals[fKey]}
	if err := ops.Edit(m.store, m.editing, p); err != nil {
		m.formErr = err.Error()
		return
	}
	m.mode = modeList
	m.reload()
	m.say(fmt.Sprintf("Updated %q", p.Name), false)
}

func indexOf(s *profile.Store, name string) int {
	for i, p := range s.Profiles {
		if p.Name == name {
			return i
		}
	}
	return 0
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if t, ok := msg.(testDoneMsg); ok {
		m.busy = ""
		if t.err != nil {
			m.say(fmt.Sprintf("%s: %v", t.name, t.err), true)
		} else {
			m.say(fmt.Sprintf("%s authenticates as @%s", t.name, t.user), false)
		}
		return m, nil
	}
	switch m.mode {
	case modeForm:
		return m.updateForm(msg)
	case modeDelete:
		if k, ok := msg.(tea.KeyMsg); ok {
			if k.String() == "y" {
				if p, ok := m.selected(); ok {
					if err := ops.Remove(m.store, p.Name); err != nil {
						m.say(err.Error(), true)
					} else {
						m.say(fmt.Sprintf("Deleted %q (its SSH key file was kept)", p.Name), false)
						if m.cursor > 0 {
							m.cursor--
						}
						m.reload()
					}
				}
			}
			m.mode = modeList
		}
		return m, nil
	}
	return m.updateList(msg)
}

func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			m.mode = modeList
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		case "tab", "down":
			m.moveFocus(1)
			return m, nil
		case "shift+tab", "up":
			m.moveFocus(-1)
			return m, nil
		case "ctrl+s":
			m.submitForm()
			return m, nil
		case "enter":
			if m.focus == len(m.inputs)-1 {
				m.submitForm()
			} else {
				m.moveFocus(1)
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	key := k.String()
	m.pub = "" // any key dismisses the public-key panel
	switch key {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.store.Profiles)-1 {
			m.cursor++
		}
	case "n":
		return m, m.openForm(nil)
	case "e":
		if p, ok := m.selected(); ok {
			return m, m.openForm(&p)
		}
	case "d":
		if _, ok := m.selected(); ok {
			m.mode = modeDelete
		}
	case "enter", " ":
		if p, ok := m.selected(); ok {
			if err := gitx.Apply(p, gitx.Global, m.store.Profiles); err != nil {
				m.say(err.Error(), true)
			} else {
				m.reload()
				m.say("Switched globally to "+p.Name, false)
			}
		}
	case "r":
		if p, ok := m.selected(); ok {
			if !m.inRepo {
				m.say("Not inside a git repository", true)
			} else if err := gitx.Apply(p, gitx.Local, m.store.Profiles); err != nil {
				m.say(err.Error(), true)
			} else {
				m.reload()
				m.say("This repository now uses "+p.Name, false)
			}
		}
	case "l":
		if p, ok := m.selected(); ok {
			if err := gitx.WriteConfigFile(m.store.ConfigFile(p), p); err != nil {
				m.say(err.Error(), true)
			} else if err := gitx.LinkDir(m.cwd, m.store.ConfigFile(p)); err != nil {
				m.say(err.Error(), true)
			} else {
				m.reload()
				m.say(fmt.Sprintf("Repos under %s will now use %s automatically", m.cwd, p.Name), false)
			}
		}
	case "u":
		if err := gitx.UnlinkDir(m.cwd); err != nil {
			m.say("No folder rule for this directory", true)
		} else {
			m.reload()
			m.say("Removed the folder rule for "+m.cwd, false)
		}
	case "t":
		if p, ok := m.selected(); ok && m.busy == "" {
			m.busy = p.Name
			m.say("Testing "+p.Name+"...", false)
			return m, func() tea.Msg {
				u, err := gitx.TestConnection(p)
				return testDoneMsg{p.Name, u, err}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(title.Render("gitswitch") + subtle.Render("  pick the account to use") + "\n\n")
	if m.mode == modeForm {
		return b.String() + m.viewForm()
	}

	g, e := m.global, m.here
	b.WriteString(subtle.Render("global    ") + identity(g) + "\n")
	b.WriteString(subtle.Render("here      ") + identity(e) + subtle.Render("  "+m.cwd) + "\n\n")

	if len(m.store.Profiles) == 0 {
		b.WriteString("No profiles yet. Press " + title.Render("n") + " to create one.\n\n")
		b.WriteString(subtle.Render("n new   q quit") + "\n")
		return b.String()
	}
	for i, p := range m.store.Profiles {
		b.WriteString(m.renderCard(i, p, g) + "\n")
	}

	if m.mode == modeDelete {
		p, _ := m.selected()
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("Delete profile %q and its folder rules? (y/N)", p.Name)) + "\n")
		return b.String()
	}
	if m.pub != "" {
		b.WriteString("\n" + title.Render("Add this public key to the account's SSH keys") + "\n")
		b.WriteString(subtle.Render("GitHub: https://github.com/settings/ssh/new") + "\n" + m.pub + "\n")
	}
	if m.msg != "" {
		c := green
		if m.msgErr {
			c = red
		}
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(c).Render(m.msg) + "\n")
	}
	b.WriteString("\n" + subtle.Render("↑/↓ move   enter switch globally   r this repo only   l auto-use for this folder   u unlink folder") + "\n")
	b.WriteString(subtle.Render("n new   e edit   d delete   t test connection   q quit") + "\n")
	return b.String()
}

func (m model) viewForm() string {
	var b strings.Builder
	if m.editing == "" {
		b.WriteString(title.Render("New profile") + "\n\n")
	} else {
		b.WriteString(title.Render("Edit profile "+m.editing) + "\n\n")
	}
	for i, f := range m.fields {
		label := fmt.Sprintf("%-10s", fieldMeta[f].label)
		if i == m.focus {
			label = lipgloss.NewStyle().Bold(true).Foreground(accent).Render(label)
		} else {
			label = subtle.Render(label)
		}
		b.WriteString(label + " " + m.inputs[i].View() + "\n")
	}
	if m.editing == "" {
		b.WriteString("\n" + subtle.Render("A new SSH key is generated for the profile automatically.") + "\n")
	}
	if m.formErr != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(red).Render(m.formErr) + "\n")
	}
	b.WriteString("\n" + subtle.Render("tab next field   enter next/save   ctrl+s save   esc cancel") + "\n")
	return b.String()
}

func identity(i gitx.Identity) string {
	if i.Email == "" {
		return subtle.Render("(not set)")
	}
	return fmt.Sprintf("%s <%s>", i.Name, i.Email)
}

func (m model) renderCard(i int, p profile.Profile, g gitx.Identity) string {
	active := strings.EqualFold(p.Email, g.Email) && p.GitName == g.Name
	marker := subtle.Render("○")
	if active {
		marker = lipgloss.NewStyle().Foreground(green).Render("●")
	}
	head := fmt.Sprintf("%s %s", marker, lipgloss.NewStyle().Bold(true).Render(p.Name))
	if active {
		head += lipgloss.NewStyle().Foreground(green).Render("  active")
	}
	lines := []string{head, subtle.Render(fmt.Sprintf("%s <%s>", p.GitName, p.Email))}
	if p.GitHub != "" {
		lines = append(lines, subtle.Render(p.Hostname()+"/"+p.GitHub))
	} else if p.Host != "" {
		lines = append(lines, subtle.Render(p.Host))
	}
	for _, l := range m.links {
		if strings.EqualFold(l.File, filepath.ToSlash(m.store.ConfigFile(p))) {
			lines = append(lines, subtle.Render("folder  "+l.Dir))
		}
	}
	style := card
	if i == m.cursor {
		style = cardSel
	}
	return style.Render(strings.Join(lines, "\n"))
}
