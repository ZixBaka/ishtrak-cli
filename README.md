# ishtrak-cli

Go CLI for [Ishtrak](https://github.com/ZixBaka/ishtrak) — create and manage tasks on your project management platform directly from the terminal, via the Ishtrak browser extension.

## How it works

The CLI talks to the [Ishtrak browser extension](https://github.com/ZixBaka/ishtrak-ext) through a local daemon (HTTP on `127.0.0.1:7474`). The extension holds the authenticated browser session and calls platform APIs on behalf of the CLI.

```
ishtrak task create  →  daemon  →  extension  →  Jira / Linear / GitHub
```

## Requirements

- Go 1.21+
- [Ishtrak browser extension](https://github.com/ZixBaka/ishtrak-ext) installed in Chrome/Firefox
- Chrome/Firefox running with the extension active

## Installation

```bash
go install github.com/zixbaka/ishtrak@latest
```

Or build from source:

```bash
git clone https://github.com/ZixBaka/ishtrak-cli
cd ishtrak-cli
make build
make install          # copies to /usr/local/bin/ishtrak
```

## Setup

```bash
ishtrak init          # interactive wizard — writes config and native host manifest
ishtrak daemon --install   # install daemon as a system service (auto-starts on login)
```

Config lives at `~/.config/ishtrak/config.toml`:

```toml
extensionId = "your-chrome-extension-id"

[defaults]
storyPattern = "([A-Z]{2,10}-[0-9]{1,6})"
taskTitleTemplate = "{storyId}: {commitSubject}"
taskDescriptionTemplate = "Commit: {commitHash}\nBranch: {branch}\n\n{commitBody}"

[platforms."jira.acme.com"]
token = "pat_xxxxxxxxxxxx"
defaultProjectId = "proj-abc123"
```

## Usage

```bash
# Task management
ishtrak task list                             # list tasks (uses first configured platform)
ishtrak task list --host jira.acme.com        # specify platform
ishtrak task create --title "Fix login bug"   # create a task
ishtrak task get PROJ-123                     # get task details
ishtrak task update PROJ-123 --status "In Progress"

# Profile management
ishtrak profile list     # list profiles learned by the extension
ishtrak profile delete jira.acme.com

# Daemon
ishtrak daemon               # run in foreground
ishtrak daemon --install     # install as launchd (macOS) / systemd (Linux) service
ishtrak daemon --uninstall   # remove service
```

## Template variables

| Variable | Description |
|----------|-------------|
| `{storyId}` | Extracted story ID (e.g. `PROJ-123`) |
| `{commitSubject}` | First line of the commit message |
| `{commitHash}` | Full commit SHA |
| `{branch}` | Current branch name |
| `{commitBody}` | Commit message body (after the first line) |

## Build commands

```bash
make build    # go build -o dist/ishtrak .
make test     # go test ./...
make lint     # go vet ./...
make install  # cp dist/ishtrak /usr/local/bin/ishtrak
make clean    # rm -rf dist/
```

## Architecture

```
cmd/               Cobra subcommands (task, profile, daemon, host, init)
internal/
  config/          TOML config loader (~/.config/ishtrak/config.toml)
  daemon/          HTTP long-poll hub (broker between CLI and extension)
  git/             Commit parser, story ID extraction, hook management
  messaging/       Native Messaging host protocol (4-byte LE framing)
  queue/           Persistent JSONL queue (~/.config/ishtrak/pending.jsonl)
  requests/        File-based request/response store
```

## Related

- [ishtrak-ext](https://github.com/ZixBaka/ishtrak-ext) — browser extension
- [ishtrak](https://github.com/ZixBaka/ishtrak) — overview and quick-start guide
