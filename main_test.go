package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
	"github.com/uzairrehman4/gitswitch/internal/profile"
)

func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GITSWITCH_HOME", filepath.Join(home, "gs"))
	return home
}

func mustRun(t *testing.T, args ...string) {
	t.Helper()
	if err := run(args); err != nil {
		t.Fatalf("gitswitch %s: %v", strings.Join(args, " "), err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestAddUseListRemove(t *testing.T) {
	isolate(t)
	mustRun(t, "add", "--name", "work", "--git-name", "Work Me", "--email", "me@work.com", "--github", "workme", "--no-key")
	mustRun(t, "add", "--name", "gl", "--git-name", "GL Me", "--email", "me@gl.com", "--github", "glme", "--host", "gitlab.com", "--no-key")

	if err := run([]string{"add", "--name", "work", "--git-name", "x", "--email", "x@y.z", "--github", "x", "--no-key"}); err == nil {
		t.Error("duplicate profile should fail")
	}
	if err := run([]string{"add", "--name", "bad", "--git-name", "x", "--email", "nope", "--github", "x", "--no-key"}); err == nil {
		t.Error("invalid email should fail")
	}

	mustRun(t, "use", "work")
	if g := gitx.GlobalIdentity(); g.Email != "me@work.com" || g.Name != "Work Me" {
		t.Fatalf("global identity = %+v", g)
	}
	mustRun(t, "use", "gl")
	if got := gitx.GetScoped(gitx.Global, "credential.https://gitlab.com.username"); got != "glme" {
		t.Errorf("gitlab username hint = %q", got)
	}
	if got := gitx.GetScoped(gitx.Global, "credential.https://github.com.username"); got != "workme" {
		// Different hosts keep independent hints; switching to gl must not clear github's.
		t.Errorf("github hint should be untouched, got %q", got)
	}

	mustRun(t, "list")
	mustRun(t, "status")
	mustRun(t, "remove", "gl")
	if err := run([]string{"use", "gl"}); err == nil {
		t.Error("removed profile should be gone")
	}
}

func TestUseRepoOnly(t *testing.T) {
	isolate(t)
	mustRun(t, "add", "--name", "work", "--git-name", "Work Me", "--email", "me@work.com", "--github", "w", "--no-key")
	mustRun(t, "add", "--name", "me", "--git-name", "Me", "--email", "me@home.com", "--github", "m", "--no-key")
	mustRun(t, "use", "me")

	repo := t.TempDir()
	gitCmd(t, repo, "init", "-q")
	chdir(t, repo)
	mustRun(t, "use", "work", "--repo")
	if got := gitCmd(t, repo, "config", "user.email"); got != "me@work.com" {
		t.Errorf("repo identity = %q", got)
	}
	if g := gitx.GlobalIdentity(); g.Email != "me@home.com" {
		t.Errorf("--repo must not change the global identity, got %q", g.Email)
	}
	chdir(t, t.TempDir())
	if err := run([]string{"use", "work", "--repo"}); err == nil {
		t.Error("--repo outside a repository should fail")
	}
}

func TestLinkAppliesFolderIdentity(t *testing.T) {
	isolate(t)
	mustRun(t, "add", "--name", "work", "--git-name", "Work Me", "--email", "me@work.com", "--github", "w", "--no-key")
	mustRun(t, "add", "--name", "me", "--git-name", "Me", "--email", "me@home.com", "--github", "m", "--no-key")
	mustRun(t, "use", "me")

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	repo := filepath.Join(workDir, "api")
	other := filepath.Join(root, "other")
	for _, d := range []string{repo, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, d, "init", "-q")
	}
	mustRun(t, "link", "work", workDir)
	if got := gitCmd(t, repo, "config", "user.email"); got != "me@work.com" {
		t.Errorf("repo under linked folder uses %q", got)
	}
	if got := gitCmd(t, other, "config", "user.email"); got != "me@home.com" {
		t.Errorf("repo outside linked folder uses %q", got)
	}
	mustRun(t, "unlink", workDir)
	if got := gitCmd(t, repo, "config", "user.email"); got != "me@home.com" {
		t.Errorf("after unlink repo uses %q", got)
	}
}

func TestEditReappliesActiveProfile(t *testing.T) {
	isolate(t)
	mustRun(t, "add", "--name", "work", "--git-name", "Work Me", "--email", "me@work.com", "--github", "w", "--no-key")
	mustRun(t, "use", "work")
	mustRun(t, "edit", "work", "--email", "new@work.com", "--github", "w2")
	if g := gitx.GlobalIdentity(); g.Email != "new@work.com" {
		t.Errorf("active profile edit not applied, global = %q", g.Email)
	}
	store, _ := profile.Load()
	if p, _ := store.Get("work"); p.GitHub != "w2" || p.Email != "new@work.com" {
		t.Errorf("stored profile = %+v", p)
	}
	if err := run([]string{"edit", "work", "--email", "broken"}); err == nil {
		t.Error("invalid edit should fail")
	}
}

func TestImportGlobalIdentity(t *testing.T) {
	isolate(t)
	if err := run([]string{"import"}); err == nil {
		t.Error("import with no global identity should fail")
	}
	gitCmd(t, ".", "config", "--global", "user.name", "Existing Me")
	gitCmd(t, ".", "config", "--global", "user.email", "existing@me.com")
	mustRun(t, "import", "main")
	store, _ := profile.Load()
	if p, ok := store.Get("main"); !ok || p.Email != "existing@me.com" {
		t.Fatalf("import failed: %+v", store.Profiles)
	}
	if err := run([]string{"import", "again"}); err == nil {
		t.Error("importing an identity that is already a profile should fail")
	}
}

// TestGuardBlocksWrongIdentity builds the real binary because the git hook
// executes it.
func TestGuardBlocksWrongIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("sh"); err != nil {
			if _, err := os.Stat(`C:\Program Files\Git\usr\bin\sh.exe`); err != nil {
				t.Skip("no sh available for git hooks")
			}
		}
	}
	// Build before isolating HOME, otherwise go starts with an empty build cache.
	bin := filepath.Join(t.TempDir(), "gitswitch")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	home := isolate(t)
	gs := func(args ...string) (string, error) {
		out, err := exec.Command(bin, args...).CombinedOutput()
		return string(out), err
	}
	must := func(args ...string) {
		t.Helper()
		if out, err := gs(args...); err != nil {
			t.Fatalf("gitswitch %v: %v\n%s", args, err, out)
		}
	}

	must("add", "--name", "work", "--git-name", "Work Me", "--email", "me@work.com", "--github", "w", "--no-key")
	must("add", "--name", "me", "--git-name", "Me", "--email", "me@home.com", "--github", "m", "--no-key")
	must("use", "me")
	workDir := filepath.Join(home, "work")
	repo := filepath.Join(workDir, "api")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, repo, "init", "-q")
	must("link", "work", workDir)
	must("guard", "install")

	// A repo-local override makes the identity disagree with the folder rule.
	gitCmd(t, repo, "config", "--local", "user.name", "Me")
	gitCmd(t, repo, "config", "--local", "user.email", "me@home.com")
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "should be blocked")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("commit with the wrong identity should be blocked:\n%s", out)
	}
	if !strings.Contains(string(out), "linked to profile") {
		t.Errorf("expected a helpful guard message, got:\n%s", out)
	}

	// Bypass works.
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "bypass")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GITSWITCH_SKIP_GUARD=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("bypass should allow the commit: %v\n%s", err, out)
	}

	// Fixing the identity lets the commit through, and repo hooks still run.
	cmd0 := exec.Command(bin, "use", "work", "--repo")
	cmd0.Dir = repo
	if out, err := cmd0.CombinedOutput(); err != nil {
		t.Fatalf("use --repo: %v\n%s", err, out)
	}
	hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho repo-hook-ran >&2\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "ok")
	cmd.Dir = repo
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit with the right identity should pass: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "repo-hook-ran") {
		t.Errorf("the repository's own pre-commit hook must still run, got:\n%s", out)
	}

	must("guard", "uninstall")
	if gitx.HooksPath() != "" {
		t.Errorf("uninstall should clear core.hooksPath, got %q", gitx.HooksPath())
	}
}
