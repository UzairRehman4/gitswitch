// Package tui is the interactive account picker.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
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
}

// Run starts the picker.
func Run(store *profile.Store) error {
	cwd, _ := os.Getwd()
	m := model{store: store, cwd: cwd, inRepo: gitx.InRepo()}
	m.reload()
	for i, p := range store.Profiles {
		if strings.EqualFold(p.Email, m.global.Email) {
			m.cursor = i
		}
	}
	_, err := tea.NewProgram(m).Run()
	return err
}

func (m *model) reload() {
	m.links = gitx.Links(m.store.ProfilesDir())
	m.global = gitx.GlobalIdentity()
	m.here = gitx.Effective()
}

func (m model) Init() tea.Cmd { return nil }

func (m model) selected() (profile.Profile, bool) {
	if len(m.store.Profiles) == 0 {
		return profile.Profile{}, false
	}
	return m.store.Profiles[m.cursor], true
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case testDoneMsg:
		m.busy = ""
		if msg.err != nil {
			m.say(fmt.Sprintf("%s: %v", msg.name, msg.err), true)
		} else {
			m.say(fmt.Sprintf("%s authenticates to GitHub as @%s", msg.name, msg.user), false)
		}
	case tea.KeyMsg:
		switch msg.String() {
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
	}
	return m, nil
}

func (m *model) say(s string, isErr bool) { m.msg, m.msgErr = s, isErr }

func (m model) View() string {
	var b strings.Builder
	b.WriteString(title.Render("gitswitch") + subtle.Render("  pick the GitHub account to use") + "\n\n")

	g, e := m.global, m.here
	b.WriteString(subtle.Render("global    ") + identity(g) + "\n")
	b.WriteString(subtle.Render("here      ") + identity(e) + subtle.Render("  "+m.cwd) + "\n\n")

	if len(m.store.Profiles) == 0 {
		b.WriteString("No profiles yet. Create one with:\n\n  gitswitch add\n\n")
		b.WriteString(subtle.Render("q quit") + "\n")
		return b.String()
	}

	for i, p := range m.store.Profiles {
		b.WriteString(m.renderCard(i, p, g) + "\n")
	}

	if m.msg != "" {
		c := green
		if m.msgErr {
			c = red
		}
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(c).Render(m.msg) + "\n")
	}
	b.WriteString("\n" + subtle.Render("↑/↓ move   enter switch globally   r this repo only   l auto-use for this folder   u unlink folder   t test   q quit") + "\n")
	return b.String()
}

func identity(i gitx.Identity) string {
	if i.Email == "" {
		return subtle.Render("(not set)")
	}
	return fmt.Sprintf("%s <%s>", i.Name, i.Email)
}

func (m model) renderCard(i int, p profile.Profile, g gitx.Identity) string {
	active := strings.EqualFold(p.Email, g.Email)
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
		lines = append(lines, subtle.Render("github.com/"+p.GitHub))
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
