// gitswitch: switch between multiple GitHub accounts, visually or from scripts.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
	"github.com/uzairrehman4/gitswitch/internal/profile"
	"github.com/uzairrehman4/gitswitch/internal/tui"
)

var version = "dev"

const usage = `gitswitch - switch between multiple GitHub accounts

Usage:
  gitswitch                     open the interactive picker
  gitswitch add [flags]         create a profile (and an SSH key for it)
  gitswitch list                list profiles
  gitswitch status              show the identity git will use right here
  gitswitch use <name> [--repo] switch globally, or only for the current repo
  gitswitch link <name> [dir]   always use <name> for repos under dir (default: .)
  gitswitch unlink [dir]        remove a folder rule
  gitswitch test <name>         check that the profile's key reaches GitHub
  gitswitch remove <name>       delete a profile (the SSH key file is kept)
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

func cmdAdd(store *profile.Store, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	name := fs.String("name", "", "profile label, e.g. work")
	gitName := fs.String("git-name", "", "git user.name")
	email := fs.String("email", "", "git user.email")
	gh := fs.String("github", "", "GitHub username")
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
		*gh = prompt(in, "GitHub username (optional)", "")
	}

	p := profile.Profile{Name: *name, GitName: *gitName, Email: *email, GitHub: *gh}
	if err := profile.Validate(p); err != nil {
		return err
	}
	if _, exists := store.Get(p.Name); exists {
		return fmt.Errorf("profile %q already exists", p.Name)
	}

	generated := false
	switch {
	case *key != "":
		p.KeyPath = *key
		if _, err := os.Stat(p.KeyPath); err != nil {
			return fmt.Errorf("key not found: %s", p.KeyPath)
		}
	case !*noKey:
		path, err := gitx.DefaultKeyPath(p.Name)
		if err != nil {
			return err
		}
		if generated, err = gitx.GenerateKey(path, p.Email); err != nil {
			return err
		}
		p.KeyPath = path
	}

	if err := store.Add(p); err != nil {
		return err
	}
	if err := store.Save(); err != nil {
		return err
	}
	if err := gitx.WriteConfigFile(store.ConfigFile(p), p); err != nil {
		return err
	}
	fmt.Printf("Added profile %q.\n", p.Name)

	if p.KeyPath != "" {
		pub, err := gitx.PublicKey(p.KeyPath)
		if err != nil {
			return err
		}
		if generated {
			fmt.Println("\nGenerated a new SSH key. Add this public key to GitHub")
		} else {
			fmt.Println("\nUsing the existing SSH key. Make sure this public key is on GitHub")
		}
		fmt.Print("(sign in as the account this profile is for): https://github.com/settings/ssh/new\n\n")
		fmt.Println(pub)
		if gitx.CopyToClipboard(pub) {
			fmt.Println("\n(copied to your clipboard)")
		}
		fmt.Printf("\nThen verify with: gitswitch test %s\n", p.Name)
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
	// Accept the flag before or after the name.
	var names []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return err
		}
		args = fs.Args()
		if len(args) > 0 {
			names = append(names, args[0])
			args = args[1:]
		}
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
	fmt.Printf("OK: %s authenticates to GitHub as @%s\n", p.Name, user)
	if p.GitHub != "" && !strings.EqualFold(p.GitHub, user) {
		fmt.Printf("warning: profile says GitHub user %q but the key belongs to @%s\n", p.GitHub, user)
	}
	return nil
}

func cmdRemove(store *profile.Store, args []string) error {
	p, err := need(store, args)
	if err != nil {
		return err
	}
	gitx.UnlinkProfile(store.ProfilesDir(), store.ConfigFile(p))
	store.Remove(p.Name)
	if err := store.Save(); err != nil {
		return err
	}
	os.Remove(store.ConfigFile(p))
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
	if problems > 0 {
		return fmt.Errorf("%d problem(s) found", problems)
	}
	return nil
}
