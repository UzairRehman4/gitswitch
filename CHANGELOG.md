# Changelog

## 0.1.2

- `gitswitch version` now reports the real version for `go install` builds instead of `dev`.
- README screenshot.

## 0.1.1

- Fix: folder rules (and the commit guard) now work when the folder is reached through an alias: Windows 8.3 short names, macOS `/var` -> `/private/var`, or a symlinked directory. Rules are stored with the real path.
- CI now runs the full test suite on Linux, macOS and Windows.

## 0.1.0

First release.

- Interactive picker: switch accounts, create, edit and delete profiles, link folders, test keys.
- Commands: `add`, `edit`, `import`, `list`, `status`, `use [--repo]`, `link`, `unlink`, `test`, `remove`, `doctor`.
- Folder rules through git's own `includeIf`, so every tool respects them.
- Per-profile ed25519 SSH keys pinned with `core.sshCommand` and `IdentitiesOnly`.
- Commit guard (`guard install`) that blocks commits made with the wrong identity and chains to each repo's own hooks.
- GitHub, GitHub Enterprise, GitLab and other SSH hosts via `--host`.
