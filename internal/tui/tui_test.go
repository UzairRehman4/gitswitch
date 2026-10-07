package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
	"github.com/uzairrehman4/gitswitch/internal/profile"
)

func testModel() model {
	s := &profile.Store{Profiles: []profile.Profile{
		{Name: "personal", GitName: "Me", Email: "me@gmail.com", GitHub: "meperso"},
		{Name: "work", GitName: "Me W", Email: "me@work.com"},
	}}
	return model{store: s, cwd: "/tmp", global: gitx.Identity{Name: "Me W", Email: "me@work.com"}}
}

func TestViewListsProfilesAndMarksActive(t *testing.T) {
	v := testModel().View()
	for _, want := range []string{"personal", "work", "me@work.com", "active", "github.com/meperso"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestCursorBounds(t *testing.T) {
	m := testModel()
	for i := 0; i < 5; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(model)
	}
	if m.cursor != 1 {
		t.Errorf("cursor should stop at last item, got %d", m.cursor)
	}
	for i := 0; i < 5; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = next.(model)
	}
	if m.cursor != 0 {
		t.Errorf("cursor should stop at first item, got %d", m.cursor)
	}
}

func TestEmptyViewHintsAdd(t *testing.T) {
	m := testModel()
	m.store = &profile.Store{}
	if !strings.Contains(m.View(), "gitswitch add") {
		t.Error("empty state should tell the user to run `gitswitch add`")
	}
}
