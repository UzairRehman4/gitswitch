# gitswitch

Switch between multiple GitHub accounts without thinking about it.

Run `gitswitch` and pick an account from a visual list, or let folder rules do it for you: everything under `~/work` commits as your work account, everything under `~/personal` as your personal one, with the right SSH key and no manual switching.

```
gitswitch  pick the GitHub account to use

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
│ folder  C:/code/work/                                        │
╰──────────────────────────────────────────────────────────────╯

↑/↓ move   enter switch globally   r this repo only   l auto-use for this folder   u unlink folder   t test   q quit
```

## What goes wrong with multiple accounts

1. Commits go out with the wrong name or email.
2. A push is rejected, or lands on the wrong account, because git used the other account's credentials or SSH key.
3. Setting up one SSH key per account by hand is fiddly.

gitswitch bundles identity (`user.name`, `user.email`), the SSH key (`core.sshCommand` with `IdentitiesOnly`), and the HTTPS username hint into one **profile**, and applies it the way git natively supports: plain `git config`, including `includeIf` folder rules. There is no daemon, no wrapper around git, and nothing to uninstall beyond the binary. Your remotes are never rewritten.

## Install

With Go 1.24+:

```
go install github.com/uzairrehman4/gitswitch@latest
```

Or download a binary from the Releases page. It runs on Windows, macOS and Linux and needs `git` and (for SSH keys) `ssh-keygen` on your PATH.

## Quick start

```
gitswitch add            # prompts for a label, name, email; generates an SSH key
gitswitch add            # ...repeat for your other account
```

`add` prints the new public key (and copies it to your clipboard). Add it to the matching GitHub account at <https://github.com/settings/ssh/new>, then check it:

```
gitswitch test work      # OK: work authenticates to GitHub as @your-work-handle
```

Then either switch by hand:

```
gitswitch                # visual picker
gitswitch use work       # global switch
gitswitch use personal --repo   # only this repository
```

or set it once and forget it:

```
gitswitch link work ~/code/work
gitswitch link personal ~/code/personal
```

## Commands

| Command | What it does |
| --- | --- |
| `gitswitch` | Interactive picker |
| `add [--name --git-name --email --github --key PATH --no-key]` | Create a profile; generates an ed25519 key unless `--key` or `--no-key` |
| `list` | List profiles, the active one, and folder rules |
| `status` | Show the identity git will use in the current directory |
| `use <name> [--repo]` | Switch globally, or just this repo |
| `link <name> [dir]` | Folder rule: repos under `dir` always use `<name>` |
| `unlink [dir]` | Remove a folder rule |
| `test <name>` | Verify the profile's key authenticates to GitHub |
| `remove <name>` | Delete a profile and its folder rules (the key file is kept) |
| `doctor` | Report missing keys, unknown identities, HTTPS remotes that bypass the key |

## How it works

- Profiles live in `profiles.json` in your user config directory (`%APPDATA%\gitswitch` on Windows, `~/.config/gitswitch` on Linux, `~/Library/Application Support/gitswitch` on macOS). Set `GITSWITCH_HOME` to move it.
- `use` writes `user.name`, `user.email`, `core.sshCommand` and `credential.https://github.com.username` into your global (or repo) git config. It only removes an SSH or credential setting if it matches one a profile put there.
- `link` adds an `includeIf "gitdir/i:<dir>/"` rule to your global config pointing at a generated per-profile file. Git evaluates it itself, so it works from any tool: terminal, VS Code, JetBrains, GUI clients.
- Private keys are generated without a passphrase so unattended pushes work. Use `--key` to point at a key you already protect, or remove the key and re-add with your own.
- SSH-only caveat: the key pinning applies to `git@github.com:...` remotes. For HTTPS remotes, gitswitch sets the GitHub username hint and relies on your credential helper (such as Git Credential Manager) to hold one login per account.

## Development

```
go test ./...
go build -o gitswitch .
```

To try it without touching your real git config:

```
GIT_CONFIG_GLOBAL=/tmp/gitconfig GITSWITCH_HOME=/tmp/gs ./gitswitch add
```

## License

MIT
