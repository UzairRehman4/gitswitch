package tui

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
	"github.com/uzairrehman4/gitswitch/internal/profile"
)

func isolate(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GITSWITCH_HOME", filepath.Join(home, "gs"))
}

// drive runs the real program with scripted keystrokes, one write per key so
// the input parser sees them as separate key presses.
func drive(t *testing.T, store *profile.Store, keys []string) {
	t.Helper()
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- RunWith(store, tea.WithInput(pr), tea.WithOutput(io.Discard), tea.WithoutRenderer())
	}()
	for _, k := range keys {
		if _, err := pw.Write([]byte(k)); err != nil {
			t.Fatalf("write %q: %v", k, err)
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("picker did not exit")
	}
}

func typed(s string) []string {
	var out []string
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}

func join(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestPickerFullFlow(t *testing.T) {
	isolate(t)
	store, err := profile.Load()
	if err != nil {
		t.Fatal(err)
	}

	newProfile := func(label, name, email string) []string {
		return join([]string{"n"}, typed(label), []string{"\t"}, typed(name), []string{"\t"}, typed(email), []string{"\t"},
			typed(label+"-user"), []string{"\t", "\r"}) // host left blank, enter on last field saves
	}
	keys := join(
		newProfile("work", "Work Me", "me@work.com"),
		newProfile("home", "Home Me", "me@home.com"),
		// list is sorted: home, work. Cursor sits on the newly added "home".
		[]string{"j", "\r", "q"}, // move to work, switch globally, quit
	)
	drive(t, store, keys)

	store, _ = profile.Load()
	if len(store.Profiles) != 2 {
		t.Fatalf("want 2 profiles created from the picker, got %+v", store.Profiles)
	}
	work, _ := store.Get("work")
	if work.KeyPath == "" {
		t.Error("a key should have been generated for the new profile")
	} else if _, err := os.Stat(work.KeyPath + ".pub"); err != nil {
		t.Errorf("public key missing: %v", err)
	}
	if g := gitx.GlobalIdentity(); g.Email != "me@work.com" || g.Name != "Work Me" {
		t.Errorf("enter should switch globally to work, got %+v", g)
	}
	if got := gitx.GetScoped(gitx.Global, "credential.https://github.com.username"); got != "work-user" {
		t.Errorf("username hint = %q", got)
	}

	// Edit "work" (cursor is on it): tab from name to email and append a character.
	drive(t, store, join(
		[]string{"e", "\t"}, typed("x"), []string{"\x13"}, // ctrl+s saves
		[]string{"q"},
	))
	store, _ = profile.Load()
	if p, _ := store.Get("work"); p.Email != "me@work.comx" {
		t.Errorf("edit not saved, email = %q", p.Email)
	}
	if g := gitx.GlobalIdentity(); g.Email != "me@work.comx" {
		t.Errorf("editing the active profile should re-apply it globally, got %q", g.Email)
	}

	// Delete "home" (move up) with confirmation.
	drive(t, store, []string{"k", "d", "y", "q"})
	store, _ = profile.Load()
	if len(store.Profiles) != 1 || store.Profiles[0].Name != "work" {
		t.Errorf("delete failed: %+v", store.Profiles)
	}
}

func TestFormRejectsInvalidInput(t *testing.T) {
	isolate(t)
	store, _ := profile.Load()
	// Label only; save with ctrl+s. Must stay in the form and create nothing.
	drive(t, store, join([]string{"n"}, typed("bad name"), []string{"\x13"}, []string{"\x03"}))
	store, _ = profile.Load()
	if len(store.Profiles) != 0 {
		t.Errorf("invalid profile was saved: %+v", store.Profiles)
	}
}
