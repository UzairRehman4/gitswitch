# Contributing

Bug reports, ideas and pull requests are welcome.

## Setup

You need Go 1.24+, `git`, and `ssh-keygen` (tests that generate keys skip themselves without it).

```
git clone https://github.com/UzairRehman4/gitswitch
cd gitswitch
go test ./...
```

## Ground rules

- Tests must never touch your real git config. Use the `isolate` helpers in `main_test.go` and `internal/tui/flow_test.go`: they point `GIT_CONFIG_GLOBAL`, `GITSWITCH_HOME`, `HOME` and `USERPROFILE` at a temp directory.
- Build the binary for end-to-end tests before isolating `HOME`, otherwise `go build` starts with an empty cache.
- Run `gofmt` and `go vet ./...` before opening a PR.
- gitswitch changes only standard git config (`user.*`, `core.sshCommand`, `core.hooksPath`, `credential.*`, `includeIf`). New features should keep it that way: no daemons, no git wrappers, no rewriting remotes.

## Layout

| Path | Purpose |
| --- | --- |
| `main.go` | CLI commands |
| `internal/profile` | Profile storage |
| `internal/gitx` | Everything that runs `git` or `ssh`: applying identities, folder rules, hooks, key tests |
| `internal/ops` | Add/edit/remove/import logic shared by the CLI and the picker |
| `internal/tui` | The interactive picker |
