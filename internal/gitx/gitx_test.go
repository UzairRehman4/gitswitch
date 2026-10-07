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
