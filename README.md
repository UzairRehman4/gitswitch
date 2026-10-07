# gitswitch

Switch between multiple GitHub accounts without thinking about it.

Run `gitswitch` and pick an account from a visual list, or let folder rules do it for you: everything under `~/work` commits as your work account, everything under `~/personal` as your personal one, each with the right SSH key. A commit guard stops the one mistake that is hard to undo: committing with the wrong identity.

```
gitswitch  pick the account to use

global    Me Personal <me@gmail.com>
here      Me W <me@work.com>  C:\code\work\api

╭──────────────────────────────────────────────────────────────╮
│ ○ personal                                                   │
│ Me Personal <me@gmail.com>                                   │
│ github.com/meperso                                           │
╰──────────────────────────────────────────────────────────────╯
╭──────────────────────────────────────────────────────────────╮
│ ● work  active                                               │
│ Me W <me@work.com>                                           │
│ github.com/meworks                                           │
│ folder  C:/code/work/                                        │
╰──────────────────────────────────────────────────────────────╯

↑/↓ move   enter switch globally   r this repo only   l auto-use for this folder   u unlink folder
n new   e edit   d delete   t test connection   q quit
```

## What goes wrong with multiple accounts

1. Commits go out with the wrong name or email.
2. A push is rejected, or lands on the wrong account, because git used the other account's credentials or SSH key.
3. Setting up one SSH key per account by hand is fiddly.

gitswitch bundles identity (`user.name`, `user.email`), the SSH key (`core.sshCommand` with `IdentitiesOnly`) and the HTTPS username hint into one **profile**, and applies it the way git natively supports: plain `git config`, including `includeIf` folder rules. There is no daemon, no wrapper around git, and nothing to uninstall beyond the binary. Your remotes are never rewritten.

It works with GitHub, GitHub Enterprise, GitLab and any other host that uses SSH keys (set `--host`).

## Install

With Go 1.24+:

```
go install github.com/uzairrehman4/gitswitch@latest
```

Or download a binary from the [Releases](https://github.com/UzairRehman4/gitswitch/releases) page. It runs on Windows, macOS and Linux and needs `git` and (for SSH keys) `ssh-keygen` on your PATH.

## Quick start

```
gitswitch import         # optional: save your current git identity as a profile
gitswitch                # then press n to create a profile (or use `gitswitch add`)
```

Creating a profile generates an SSH key and shows the public key (it is also copied to your clipboard). Add it to the matching account at <https://github.com/settings/ssh/new> while signed in as that account, then press `t` in the picker, or run:

```
gitswitch test work      # OK: work authenticates to github.com as @your-work-handle
```

Then either switch by hand:

```
gitswitch                       # visual picker
gitswitch use work              # global switch
gitswitch use personal --repo   # only this repository
```

or set it once and forget it:

```
gitswitch link work ~/code/work
gitswitch link personal ~/code/personal
gitswitch guard install         # block commits made with the wrong identity
```

## Commands

| Command | What it does |
| --- | --- |
| `gitswitch` | Interactive picker: switch, add, edit, delete, link folders, test keys |
| `add [--name --git-name --email --github --host --key PATH --no-key]` | Create a profile; generates an ed25519 key unless `--key` or `--no-key` |
| `edit <name> [--git-name --email --github --host --key PATH --no-key]` | Change a profile. If it is the active one, the change is applied immediately |
| `import [label]` | Save the current global git identity as a profile |
| `list` | List profiles, the active one, and folder rules |
| `status` | Show the identity git will use in the current directory |
| `use <name> [--repo]` | Switch globally, or just this repo |
| `link <name> [dir]` | Folder rule: repos under `dir` always use `<name>` |
| `unlink [dir]` | Remove a folder rule |
| `test <name>` | Verify the profile's key authenticates to its host |
| `remove <name>` | Delete a profile and its folder rules (the key file is kept) |
| `guard install` / `guard uninstall` | Install or remove the commit guard |
| `doctor` | Report missing keys, unknown identities, HTTPS remotes that bypass the key |

## Commit guard

`gitswitch guard install` sets a global `core.hooksPath` with a `pre-commit` hook. If the repository sits under a folder rule but the identity git is about to use does not match that rule's profile (for example a repo-local `user.email` override), the commit is refused with the fix:

```
error: this folder is linked to profile "work" (me@work.com) but you are about to commit as me@home.com.
Fix it with `gitswitch use work --repo`, or bypass with GITSWITCH_SKIP_GUARD=1
```

Hooks that live in a repository's own `.git/hooks` keep running: every hook name gets a stub that chains to the repo's hook. `git commit --no-verify` bypasses the guard like any pre-commit hook. `guard install` refuses to replace a `core.hooksPath` you already set unless you pass `--force`.

## How it works

- Profiles live in `profiles.json` in your user config directory (`%APPDATA%\gitswitch` on Windows, `~/.config/gitswitch` on Linux, `~/Library/Application Support/gitswitch` on macOS). Set `GITSWITCH_HOME` to move it.
- `use` writes `user.name`, `user.email`, `core.sshCommand` and `credential.https://<host>.username` into your global (or repo) git config. It only removes an SSH or credential setting if it matches one a profile put there.
- `link` adds an `includeIf "gitdir/i:<dir>/"` rule to your global config pointing at a generated per-profile file. Git evaluates it itself, so it works from any tool: terminal, VS Code, JetBrains, GUI clients.
- Private keys are generated without a passphrase so unattended pushes work, and are created with owner-only permissions. Use `--key` to point at a key you already protect.
- The key pinning applies to SSH remotes (`git@github.com:owner/repo.git`). For HTTPS remotes, gitswitch sets the username hint and relies on your credential helper (such as Git Credential Manager) to hold one login per account. `gitswitch doctor` flags HTTPS remotes in repos that have a key.

## Development

```
go test ./...
go build -o gitswitch .
```

The tests run against throwaway git configs and home directories, never your real ones. They include scripted runs of the interactive picker and an end-to-end test of the commit guard with a real built binary.

To try it by hand without touching your real git config:

```
GIT_CONFIG_GLOBAL=/tmp/gitconfig GITSWITCH_HOME=/tmp/gs ./gitswitch
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT
