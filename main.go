// gitswitch: switch between multiple GitHub accounts, visually or from scripts.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
	"github.com/uzairrehman4/gitswitch/internal/ops"
	"github.com/uzairrehman4/gitswitch/internal/profile"
	"github.com/uzairrehman4/gitswitch/internal/tui"
)

// version is set with -ldflags for release binaries. For `go install` builds
// it falls back to the module version recorded in the binary.
var version = "dev"

func init() {
	if version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
}

const usage = `gitswitch - switch between multiple GitHub accounts

Usage:
  gitswitch                     open the interactive picker
  gitswitch add [flags]         create a profile (and an SSH key for it)
  gitswitch edit <name> [flags] change a profile's email, username, host or key
  gitswitch import [label]      save the current global git identity as a profile
  gitswitch list                list profiles
  gitswitch status              show the identity git will use right here
  gitswitch use <name> [--repo] switch globally, or only for the current repo
  gitswitch link <name> [dir]   always use <name> for repos under dir (default: .)
  gitswitch unlink [dir]        remove a folder rule
  gitswitch test <name>         check that the profile's key reaches GitHub
  gitswitch remove <name>       delete a profile (the SSH key file is kept)
  gitswitch guard install       block commits made with the wrong identity
  gitswitch guard uninstall     remove the commit guard
  gitswitch doctor              look for common misconfigurations
  gitswitch version
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if _, err := lookGit(); err != nil {
		return err
	}
	store, err := profile.Load()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return tui.Run(store)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "add":
		return cmdAdd(store, rest)
	case "edit":
		return cmdEdit(store, rest)
	case "import":
		return cmdImport(store, rest)
	case "guard":
		return cmdGuard(store, rest)
	case "list", "ls":
		return cmdList(store)
	case "status", "st":
		return cmdStatus(store)
	case "use":
		return cmdUse(store, rest)
	case "link":
		return cmdLink(store, rest)
	case "unlink":
		return cmdUnlink(rest)
	case "test":
		return cmdTest(store, rest)
	case "remove", "rm":
		return cmdRemove(store, rest)
	case "doctor":
		return cmdDoctor(store)
	case "version", "--version", "-v":
		fmt.Println("gitswitch", version)
		return nil
	case "help", "--help", "-h":
		fmt.Print(usage)
		return nil
	}
	fmt.Fprint(os.Stderr, usage)
	return fmt.Errorf("unknown command %q", cmd)
}

func need(store *profile.Store, args []string) (profile.Profile, error) {
	if len(args) < 1 {
		return profile.Profile{}, fmt.Errorf("profile name required (see `gitswitch list`)")
	}
	p, ok := store.Get(args[0])
	if !ok {
		return p, fmt.Errorf("no profile named %q (see `gitswitch list`)", args[0])
	}
	return p, nil
}

func prompt(r *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, _ := r.ReadString('\n')
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return def
}

// parseInterleaved parses flags that may appear before or after positionals.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) > 0 {
			pos = append(pos, args[0])
			args = args[1:]
		}
	}
	return pos, nil
}

func cmdAdd(store *profile.Store, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	name := fs.String("name", "", "profile label, e.g. work")
	gitName := fs.String("git-name", "", "git user.name")
	email := fs.String("email", "", "git user.email")
	gh := fs.String("github", "", "account username on the host")
	host := fs.String("host", "", "git host (default github.com; also gitlab.com, GitHub Enterprise, ...)")
	key := fs.String("key", "", "use an existing SSH private key instead of generating one")
	noKey := fs.Bool("no-key", false, "do not use an SSH key (HTTPS only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	in := bufio.NewReader(os.Stdin)
	g := gitx.GlobalIdentity()
	if *name == "" {
		*name = prompt(in, "Profile label (e.g. work, personal)", "")
	}
	if *gitName == "" {
		*gitName = prompt(in, "Git name", g.Name)
	}
	if *email == "" {
		*email = prompt(in, "Git email", g.Email)
	}
	if *gh == "" {
		*gh = prompt(in, "Account username (optional)", "")
	}

	p := profile.Profile{Name: *name, GitName: *gitName, Email: *email, GitHub: *gh, Host: *host, KeyPath: *key}
	res, err := ops.Add(store, p, !*noKey)
	if err != nil {
		return err
	}
	p = res.Profile
	fmt.Printf("Added profile %q.\n", p.Name)

	if res.PublicKey != "" {
		if res.Generated {
			fmt.Println("\nGenerated a new SSH key. Add this public key to the account's SSH keys")
		} else {
			fmt.Println("\nUsing an existing SSH key. Make sure this public key is on the account")
		}
		fmt.Printf("(sign in as the account this profile is for; GitHub: https://github.com/settings/ssh/new)\n\n%s\n", res.PublicKey)
		if gitx.CopyToClipboard(res.PublicKey) {
			fmt.Println("\n(copied to your clipboard)")
		}
		fmt.Printf("\nThen verify with: gitswitch test %s\n", p.Name)
	}
	return nil
}

func cmdEdit(store *profile.Store, args []string) error {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	gitName := fs.String("git-name", "", "new git user.name")
	email := fs.String("email", "", "new git user.email")
	gh := fs.String("github", "", "new account username")
	host := fs.String("host", "", "new git host")
	key := fs.String("key", "", "new SSH private key path")
	noKey := fs.Bool("no-key", false, "stop using an SSH key")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	p, err := need(store, pos)
	if err != nil {
		return err
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "git-name":
			p.GitName = *gitName
		case "email":
			p.Email = *email
		case "github":
			p.GitHub = *gh
		case "host":
			p.Host = *host
		case "key":
			p.KeyPath = *key
		case "no-key":
			if *noKey {
				p.KeyPath = ""
			}
		}
	})
	if err := ops.Edit(store, p.Name, p); err != nil {
		return err
	}
	fmt.Printf("Updated profile %q.\n", p.Name)
	return nil
}

func cmdImport(store *profile.Store, args []string) error {
	label := "default"
	if len(args) > 0 {
		label = args[0]
	}
	p, err := ops.ImportGlobal(store, label)
	if err != nil {
		return err
	}
	fmt.Printf("Saved the current global identity as profile %q (%s <%s>).\n", p.Name, p.GitName, p.Email)
	fmt.Printf("It has no SSH key yet; attach one with: gitswitch edit %s --key PATH\n", p.Name)
	return nil
}

func hooksDir(store *profile.Store) string {
	return filepath.Join(filepath.Dir(store.ProfilesDir()), "hooks")
}

func cmdGuard(store *profile.Store, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gitswitch guard install|uninstall|check")
	}
	dir := hooksDir(store)
	switch args[0] {
	case "install":
		force := len(args) > 1 && args[1] == "--force"
		if cur := gitx.HooksPath(); cur != "" && !gitx.SamePath(cur, dir) && !force {
			return fmt.Errorf("core.hooksPath is already set to %s; re-run with --force to replace it", cur)
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		if err := gitx.InstallHooks(dir, exe); err != nil {
			return err
		}
		if err := gitx.SetHooksPath(dir); err != nil {
			return err
		}
		fmt.Println("Guard installed. Commits are blocked when the identity does not match the folder rule.")
		fmt.Println("Repo hooks keep working. Bypass once with GITSWITCH_SKIP_GUARD=1 or git commit --no-verify.")
		return nil
	case "uninstall":
		if cur := gitx.HooksPath(); cur != "" && gitx.SamePath(cur, dir) {
			if err := gitx.UnsetHooksPath(); err != nil {
				return err
			}
		}
		os.RemoveAll(dir)
		fmt.Println("Guard removed.")
		return nil
	case "check":
		return guardCheck(store)
	}
	return fmt.Errorf("unknown guard command %q", args[0])
}

// guardCheck runs from the pre-commit hook.
func guardCheck(store *profile.Store) error {
	if os.Getenv("GITSWITCH_SKIP_GUARD") != "" {
		return nil
	}
	e := gitx.Effective()
	if e.Name == "" || e.Email == "" {
		return fmt.Errorf("no git identity is set here; run `gitswitch use <name> --repo`")
	}
	root, err := gitx.RepoRoot()
	if err != nil {
		return nil
	}
	rule, ok := gitx.ExpectedFor(root, gitx.Links(store.ProfilesDir()))
	if !ok {
		return nil
	}
	for _, p := range store.Profiles {
		if strings.EqualFold(filepath.ToSlash(store.ConfigFile(p)), rule.File) && !strings.EqualFold(p.Email, e.Email) {
			return fmt.Errorf("this folder is linked to profile %q (%s) but you are about to commit as %s.\nFix it with `gitswitch use %s --repo`, or bypass with GITSWITCH_SKIP_GUARD=1",
				p.Name, p.Email, e.Email, p.Name)
		}
	}
	return nil
}

func cmdList(store *profile.Store) error {
	if len(store.Profiles) == 0 {
		fmt.Println("No profiles yet. Run `gitswitch add`.")
		return nil
	}
	g := gitx.GlobalIdentity()
	links := gitx.Links(store.ProfilesDir())
	for _, p := range store.Profiles {
		mark := " "
		if strings.EqualFold(p.Email, g.Email) {
			mark = "*"
		}
		fmt.Printf("%s %-12s %s <%s>\n", mark, p.Name, p.GitName, p.Email)
		for _, l := range links {
			if strings.EqualFold(l.File, filepath.ToSlash(store.ConfigFile(p))) {
				fmt.Printf("    folder: %s\n", l.Dir)
			}
		}
	}
	fmt.Println("\n* = active globally")
	return nil
}

func cmdStatus(store *profile.Store) error {
	e := gitx.Effective()
	if e.Email == "" {
		fmt.Println("No git identity configured here.")
		return nil
	}
	label := "(not a gitswitch profile)"
	if p, ok := store.MatchIdentity(e.Name, e.Email); ok {
		label = "profile: " + p.Name
	}
	fmt.Printf("%s <%s>\n%s\n", e.Name, e.Email, label)
	if cmd := gitx.Get("core.sshCommand"); cmd != "" {
		fmt.Println("ssh:", cmd)
	}
	return nil
}

func cmdUse(store *profile.Store, args []string) error {
	fs := flag.NewFlagSet("use", flag.ContinueOnError)
	repo := fs.Bool("repo", false, "apply only to the current repository")
	names, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	p, err := need(store, names)
	if err != nil {
		return err
	}
	scope := gitx.Global
	if *repo {
		if !gitx.InRepo() {
			return fmt.Errorf("--repo needs to be run inside a git repository")
		}
		scope = gitx.Local
	}
	if err := gitx.Apply(p, scope, store.Profiles); err != nil {
		return err
	}
	where := "globally"
	if *repo {
		where = "for this repository"
	}
	fmt.Printf("Now using %s (%s <%s>) %s.\n", p.Name, p.GitName, p.Email, where)
	return nil
}

func cmdLink(store *profile.Store, args []string) error {
	p, err := need(store, args)
	if err != nil {
		return err
	}
	dir := "."
	if len(args) > 1 {
		dir = args[1]
	}
	if err := gitx.WriteConfigFile(store.ConfigFile(p), p); err != nil {
		return err
	}
	if err := gitx.LinkDir(dir, store.ConfigFile(p)); err != nil {
		return err
	}
	abs, _ := filepath.Abs(dir)
	fmt.Printf("Repos under %s will now use %s automatically.\n", abs, p.Name)
	return nil
}

func cmdUnlink(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	if err := gitx.UnlinkDir(dir); err != nil {
		return fmt.Errorf("no folder rule for %s", dir)
	}
	fmt.Println("Folder rule removed.")
	return nil
}

func cmdTest(store *profile.Store, args []string) error {
	p, err := need(store, args)
	if err != nil {
		return err
	}
	user, err := gitx.TestConnection(p)
	if err != nil {
		return err
	}
	fmt.Printf("OK: %s authenticates to %s as @%s\n", p.Name, p.Hostname(), user)
	if p.GitHub != "" && !strings.EqualFold(p.GitHub, user) {
		fmt.Printf("warning: profile says user %q but the key belongs to @%s\n", p.GitHub, user)
	}
	return nil
}

func cmdRemove(store *profile.Store, args []string) error {
	p, err := need(store, args)
	if err != nil {
		return err
	}
	if err := ops.Remove(store, p.Name); err != nil {
		return err
	}
	fmt.Printf("Removed profile %q. Its SSH key file was left in place.\n", p.Name)
	return nil
}

func cmdDoctor(store *profile.Store) error {
	problems := 0
	ok := func(f string, a ...any) { fmt.Printf("[ ok ] "+f+"\n", a...) }
	bad := func(f string, a ...any) { problems++; fmt.Printf("[FAIL] "+f+"\n", a...) }
	warn := func(f string, a ...any) { fmt.Printf("[warn] "+f+"\n", a...) }

	if len(store.Profiles) == 0 {
		warn("no profiles yet; run `gitswitch add`")
	}
	for _, p := range store.Profiles {
		if p.KeyPath == "" {
			warn("%s: no SSH key (HTTPS only)", p.Name)
			continue
		}
		if _, err := os.Stat(p.KeyPath); err != nil {
			bad("%s: SSH key missing at %s", p.Name, p.KeyPath)
		} else if _, err := gitx.PublicKey(p.KeyPath); err != nil {
			warn("%s: no .pub file next to the key", p.Name)
		} else {
			ok("%s: SSH key present", p.Name)
		}
	}

	g := gitx.GlobalIdentity()
	if g.Email == "" {
		warn("no global git identity set; commits outside folder rules will fail")
	} else if p, found := store.MatchIdentity(g.Name, g.Email); found {
		ok("global identity is profile %q", p.Name)
	} else {
		warn("global identity %s <%s> is not a gitswitch profile", g.Name, g.Email)
	}

	if gitx.InRepo() {
		e := gitx.Effective()
		p, found := store.MatchIdentity(e.Name, e.Email)
		if found {
			ok("this repo commits as profile %q", p.Name)
		} else {
			warn("this repo commits as %s <%s>, which is not a gitswitch profile", e.Name, e.Email)
		}
		if remote, err := remoteURL(); err == nil && found {
			if strings.HasPrefix(remote, "https://") && p.KeyPath != "" {
				warn("remote is HTTPS, so the SSH key is unused; switch with `git remote set-url origin git@github.com:OWNER/REPO.git`")
			}
		}
	}
	if dir := filepath.Join(filepath.Dir(store.ProfilesDir()), "hooks"); gitx.HooksPath() != "" && gitx.SamePath(gitx.HooksPath(), dir) {
		ok("commit guard is installed")
	} else {
		warn("commit guard not installed (`gitswitch guard install` blocks commits with the wrong identity)")
	}
	if problems > 0 {
		return fmt.Errorf("%d problem(s) found", problems)
	}
	return nil
}
