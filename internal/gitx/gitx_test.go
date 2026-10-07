package gitx

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/uzairrehman4/gitswitch/internal/profile"
)

func TestParseLinks(t *testing.T) {
	out := "includeif.gitdir/i:C:/Users/me/work/.path C:/Users/me/AppData/Roaming/gitswitch/profiles/work.gitconfig\n" +
		"includeif.gitdir:~/other/.path ~/somebody-elses.gitconfig\n" +
		"includeif.gitdir/i:C:/Users/me/oss/.path c:/users/me/appdata/roaming/gitswitch/profiles/oss.gitconfig"
	links := parseLinks(out, `C:\Users\me\AppData\Roaming\gitswitch\profiles`)
	if len(links) != 2 {
		t.Fatalf("want 2 gitswitch links, got %d: %+v", len(links), links)
	}
	if links[0].Dir != "C:/Users/me/work/" {
		t.Errorf("dir = %q", links[0].Dir)
	}
}

func TestLinkKey(t *testing.T) {
	if got := linkKey(`C:\Users\me\work`); got != "includeIf.gitdir/i:C:/Users/me/work/.path" {
		t.Errorf("got %q", got)
	}
}

func TestSSHCommand(t *testing.T) {
	want := `ssh -i "C:/Users/me/.ssh/k" -o IdentitiesOnly=yes`
	if got := SSHCommand(`C:\Users\me\.ssh\k`); got != want && got != `ssh -i "C:\Users\me\.ssh\k" -o IdentitiesOnly=yes` {
		t.Errorf("got %q", got)
	}
}

func TestWriteConfigFileKeyWithSpaces(t *testing.T) {
	dir := t.TempDir()
	cfg := dir + "/p.gitconfig"
	key := dir + "/my keys/id"
	if err := WriteConfigFile(cfg, profile.Profile{Name: "w", GitName: "N", Email: "e@x.com", KeyPath: key}); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "config", "--file", cfg, "core.sshCommand").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(out)), SSHCommand(key); got != want {
		t.Errorf("round trip through git config changed the value:\n got %q\nwant %q", got, want)
	}
}

func TestExpectedFor(t *testing.T) {
	links := []Link{
		{Dir: "C:/code/", File: "a.gitconfig"},
		{Dir: "C:/code/work/", File: "b.gitconfig"},
	}
	if l, ok := ExpectedFor(`c:\Code\Work\api`, links); !ok || l.File != "b.gitconfig" {
		t.Errorf("most specific rule should win, got %+v %v", l, ok)
	}
	if l, ok := ExpectedFor("C:/code/other", links); !ok || l.File != "a.gitconfig" {
		t.Errorf("got %+v %v", l, ok)
	}
	if _, ok := ExpectedFor("C:/codex/app", links); ok {
		t.Error("C:/codex must not match the C:/code/ rule")
	}
}

func TestHookScriptChainsRepoHook(t *testing.T) {
	pc := hookScript("pre-commit", `C:\bin\gitswitch.exe`)
	if !strings.Contains(pc, `"C:/bin/gitswitch.exe" guard check || exit 1`) {
		t.Error("pre-commit must run the guard")
	}
	if !strings.Contains(pc, "--git-common-dir") || strings.Contains(pc, "--git-path") {
		t.Error("pre-commit must chain to the repo's own hook")
	}
	if strings.Contains(hookScript("commit-msg", "x"), "guard check") {
		t.Error("only pre-commit runs the guard")
	}
}

func TestCredentialKeyUsesHost(t *testing.T) {
	if got := credKey("gitlab.com"); got != "credential.https://gitlab.com.username" {
		t.Error(got)
	}
	dir := t.TempDir()
	cfg := dir + "/p.gitconfig"
	p := profile.Profile{Name: "w", GitName: "N", Email: "e@x.com", GitHub: "me", Host: "git.corp.example"}
	if err := WriteConfigFile(cfg, p); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command("git", "config", "--file", cfg, credKey("git.corp.example")).Output()
	if strings.TrimSpace(string(out)) != "me" {
		t.Errorf("host-specific username not written, got %q", out)
	}
}

func TestGreetingParsing(t *testing.T) {
	for in, want := range map[string]string{
		"Hi octocat! You've successfully authenticated, but GitHub does not provide shell access.": "octocat",
		"Welcome to GitLab, @tanuki!": "tanuki",
		"authenticated via ssh key.\nYou can use git to connect to Bitbucket. Shell access is disabled\nlogged in as dev-user.": "dev-user",
	} {
		m := helloRe.FindStringSubmatch(in)
		got := ""
		for _, g := range m[1:] {
			if g != "" {
				got = g
			}
		}
		if got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
