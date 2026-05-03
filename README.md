# Sessions

Sessions is a small, editor-agnostic CLI for people who keep several Git worktrees and agent terminals alive at the same time.

It is not trying to be an IDE, an agent runner, a terminal multiplexer, or a project manager. It is a boring bit of glue around `git worktree`: create an isolated checkout, give it a stable local index, let the repo do its own setup, and let agents report when they need attention.

You probably do not need this. If you work in one branch at a time, or if your editor/agent already owns your whole workflow, plain Git plus your editor is simpler. This exists because my day often looks like this:

- a few parallel branches across one or more repos
- one VS Code or Cursor or Zed window per worktree
- one or more Claude/Codex terminals in each window
- per-worktree ports, databases, Redis namespaces, and local setup
- a lot of diff review, because I still like reading (some of) the code

Sessions keeps that workflow from turning into a pile of unnamed windows and colliding local services.

## What It Does

Sessions wraps a few pieces that `git worktree` intentionally does not handle:

- creates named worktrees with safe branch/path defaults
- assigns a stable repo-local `SESSION_INDEX` for ports, DB names, Docker project names, etc.
- writes `.env.sessions` into each worktree with generic session facts
- runs optional repo-owned create/remove hooks
- stores agent state and optional focus targets in a local registry
- lets agents call `sessions agent set` from hooks
- routes `sessions focus` to the best session or attached agent target
- runs configured attention commands for states like `needs-input` and `failed`
- refuses to remove dirty worktrees unless forced

The important boundary: Sessions owns generic lifecycle and state. Your repo owns what that means.

For example, Sessions can say:

```env
SESSION_NAME="billing"
SESSION_INDEX="3"
SESSION_WORKTREE="/Users/you/code/myapp-sessions/billing"
```

Your repo decides that index `3` means:

```env
PORT=3030
DATABASE_URL=postgres://localhost/myapp_dev_3
COMPOSE_PROJECT_NAME=myapp_3
```

## Why This Shape

The design comes from a few bets.

**Git worktrees are the right isolation primitive.** They give each branch its own checkout, editor window, terminal set, and local files without constant stash/switch churn.

**The editor window is already a good boundary.** VS Code, Zed, and other editors naturally scope tabs, search, problems, terminals, and diff review per window. Sessions does not try to virtualize that inside one giant workspace.

**The unit is the work context, not the agent.** A session may have Claude, Codex, or another agent running inside it. The agent is a child process reporting state; the session is the line of work.

**The core should be editor-agnostic.** I might use VS Code today and Zed later. The expensive part is worktree lifecycle, environment identity, and attention state. Editor integrations can come later.

**Repos know their own environment.** A generic tool should not know how your app maps indexes to ports, database names, Redis DBs, Docker Compose projects, Prisma setup, or npm install policy. It should provide stable facts and get out of the way.

## Status

This is v0 software. It is built for early hands-on use on macOS and Linux. The CLI is usable, but the public API, config shape, and registry format should still be considered early.

Windows support is out of scope for v0 because registry locking currently uses Unix `flock`.

## Requirements

- Git 2.17 or newer
- Go 1.22 or newer
- macOS or Linux

Sessions does not own desktop notification rendering. Configure an attention command if you want a repo-owned script to show banners, play sounds, persist events, or wire click behavior.

## Install

From a checkout of this repository:

```sh
make install
```

By default this installs `sessions` to:

```sh
~/.local/bin/sessions
```

Make sure the install directory is on your `PATH`:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Override the install target with `BINDIR`, `PREFIX`, or `GOBIN`:

```sh
BINDIR="$HOME/bin" make install
PREFIX=/opt/local make install
GOBIN="$HOME/go/bin" make install
```

For development, update and reinstall with:

```sh
git pull
make install
```

You can also run directly from source:

```sh
go run ./cmd/sessions --version
go run ./cmd/sessions help
```

## Quickstart

From a Git repository:

```sh
sessions init
sessions new billing --base main --editor code
```

`init` writes starter repo config and no-op hooks if the repo does not already have them. `new` creates a branch/worktree, writes `.env.sessions`, records the session in the registry, runs the repo create hook if configured, and opens the configured editor.

List active sessions:

```sh
sessions ls
```

Example output:

```text
repo      name     idx  branch    agents                         updated
myapp     billing  3    billing   claude:ready codex:running     2m ago
myapp     search   4    search    codex:needs-input              10s ago
myapp     cleanup  5    cleanup   -                              1m ago
```

Update the current session from an agent hook or shell:

```sh
sessions agent set codex --state needs-input --message "Codex needs input"
```

Focus the next session that needs attention:

```sh
sessions focus --next
```

Remove a clean session:

```sh
sessions rm billing
```

Dirty worktrees are refused unless you pass `--force`.

## Commands

### `sessions init`

```sh
sessions init
```

Initializes the current Git repo for Sessions. If `.sessions.toml` is missing,
it creates:

```text
.sessions.toml
.sessions/create.sh
.sessions/remove.sh
```

The generated hooks are no-ops. Existing config and hook files are not
overwritten.

### `sessions new`

```sh
sessions new <name> [--base <ref>] [--branch <branch>] [--editor code|cursor|zed|none] [--no-setup]
```

Creates a Git worktree and branch from the current repo.

Session names must match:

```text
^[a-z0-9][a-z0-9-]{0,62}$
```

By default, the branch name is the session name. The base ref comes from:

1. `--base`
2. repo config `base_ref`
3. default-branch discovery

Sessions never silently bases a new session on your current feature branch.

### `sessions ls`

```sh
sessions ls [--json]
```

Shows active sessions, compact agent state, and last update time. When run inside a registered repo, output is scoped to that repo; otherwise it shows all known repos.

### `sessions agent set`

```sh
sessions agent set <agent> --state running|needs-input|ready|failed|idle [--message <text>] [--session <session>]
```

Updates one agent's state.

Agent names are open-ended strings:

```sh
sessions agent set claude --state ready
sessions agent set codex --state needs-input
sessions agent set aider --state failed
sessions agent set gemini --state running
```

States are closed so Sessions can decide priority and attention behavior:

- `needs-input`: agent is blocked on user input or approval
- `failed`: agent or integration failed
- `ready`: agent completed a turn or has work ready for review
- `running`: agent started or resumed work
- `idle`: agent is no longer active

By default, `needs-input` and `failed` are attention states. `ready`, `running`, and `idle` update registry state quietly unless configured in `attention.states`.

If `--session` is not provided, Sessions resolves the current session from the current working directory, then from `SESSION_ID`. That is what makes generic Claude/Codex hooks work from inside a worktree.

### `sessions agent attach`

```sh
sessions agent attach <agent> --provider code|cursor|zed|custom --target <id> [--session <session>] [--pid <pid>] [--ttl <duration>] [--metadata <json>]
```

Stores an optional focus target for an agent. Attachments are best-effort hints for exact focus. They are ignored when stale, but the session editor fallback still works.

### `sessions agent show`

```sh
sessions agent show <agent> [--session <session>] [--json]
```

Shows agent state, message, focus target details, and whether the target is stale.

### `sessions focus`

```sh
sessions focus <session> [--agent <agent>]
sessions focus --next [--all]
```

Focuses a session or agent. `--next` selects agents in configured attention states, then by agent state priority, then by oldest update time. Inside a registered repo, `--next` searches that repo; outside a repo it searches all repos. `--all` always searches all repos.

### `sessions rm`

```sh
sessions rm [<name>] [--force]
```

Removes a session worktree and then removes its registry entry. If no name is
provided, Sessions resolves the current session from the current working
directory. Without `--force`, dirty worktrees are refused. Upstream/unpushed
branch state is reported as a warning.

After the worktree is removed, Sessions tries to delete the session branch with
Git's safe branch deletion (`git branch -d`). This makes branch names reusable
when Git can prove the branch was merged, but keeps the branch and prints a
warning when the branch has commits that are not reachable from the main branch.
`--force` only forces worktree cleanup; it does not force-delete branch history.

If an `on_remove` hook fails, removal stops unless `--force` is set.

## Configuration

Global config lives at:

```text
~/.sessions/config.toml
```

Sessions keeps all user-level files in `~/.sessions`:

- `config.toml`: global config
- `registry.json`: session registry
- `trusted.json`: trusted repo hook decisions

Example global config:

```toml
default_editor = "code"
worktree_path = "../{repo}-{name}"

[attention]
states = ["needs-input", "failed"]

[attention.command]
command = ""

[attention.command.states.failed]
command = ""

[focus.providers.custom]
command = ""

[editors.code]
command = "code"
args = ["--new-window", "{path}"]

[editors.cursor]
command = "cursor"
args = ["--new-window", "{path}"]

[editors.zed]
command = "zed"
args = ["{path}"]
```

Repo config lives at `.sessions.toml` in the repo root:

```toml
name = "sampleapp"
default_editor = "code"
worktree_path = "../{repo}-sessions/{name}"
base_ref = "main"

on_create = 'sh "$SESSION_REPO_ROOT/.sessions/create.sh"'
on_remove = 'sh "$SESSION_REPO_ROOT/.sessions/remove.sh"'

[attention]
states = ["needs-input", "failed"]

[attention.command]
command = 'sh "$SESSION_REPO_ROOT/.sessions/attention.sh"'

[focus.providers.custom]
command = 'sh "$SESSION_REPO_ROOT/.sessions/focus.sh"'

[editors.cursor]
command = "cursor"
args = ["--new-window", "{path}"]
close_command = "cursor"
close_args = ["--remove", "{path}"]
```

Repo config is an overlay for global config: `default_editor`, `worktree_path`, `attention`, `focus`, and `editors` may all be defined globally or overridden per repo. Repo-only fields such as `name`, `base_ref`, `on_create`, and `on_remove` only live in `.sessions.toml`. Editor `close_command` is best-effort and runs during `sessions rm` before the worktree is deleted.

Config precedence is:

1. CLI flags
2. repo `.sessions.toml`
3. global `~/.sessions/config.toml`
4. built-in defaults

`worktree_path` supports:

- `{repo}`
- `{name}`
- `{branch}`
- `{index}`

Relative paths are resolved from the repo's main worktree.

See [docs/config.md](docs/config.md) for more detail.
See [docs/focus.md](docs/focus.md) for hotkeys, attachment semantics, provider boundaries, and click-to-focus recipes.

## Session Env File

Each new session gets a `.env.sessions` file in the worktree:

```env
SESSION_ID="..."
SESSION_NAME="billing"
SESSION_INDEX="3"
SESSION_REPO_NAME="sampleapp"
SESSION_REPO_ROOT="/path/to/main"
SESSION_WORKTREE="/path/to/sampleapp-sessions/billing"
SESSION_BRANCH="billing"
SESSION_BASE_REF="main"
```

This file is pure dotenv-style key/value data. It intentionally does not use `export` so more tools can parse it.

Sessions excludes `.env.sessions` from Git and ignores it in dirty-worktree checks.

Repos decide how to consume these facts. For example, a repo that already uses dotenv files can layer them:

```sh
dotenv -e .env.sessions -e .env.session.development -e .env.development -- npm run dev
```

A shell script can source it with automatic export:

```sh
set -a
. ./.env.sessions
set +a
```

A direnv repo can load it from `.envrc`:

```sh
dotenv_if_exists .env.sessions
```

The intended pattern is:

1. Sessions writes generic facts to `.env.sessions`.
2. Your repo's `on_create` hook reads those facts.
3. Your repo generates whatever app-specific env files it needs.
4. Your normal dev/test commands load those app-specific env files.

See [docs/repo-setup.md](docs/repo-setup.md) for examples.

## Agent Hooks

Sessions works with any agent or script that can run `sessions agent set` from inside the worktree.

Examples:

```sh
sessions agent set claude --state needs-input --message "Claude needs input"
sessions agent set claude --state ready --message "Claude finished"
sessions agent set codex --state ready --message "Codex finished"
```

Claude and Codex proof-of-concept snippets are in:

- [docs/hooks.md](docs/hooks.md)
- [examples/claude/settings.json](examples/claude/settings.json)
- [examples/codex/config.toml](examples/codex/config.toml)

## Hook Trust

Repo hooks execute repo-controlled code. Before Sessions runs `on_create` or `on_remove` from a repo config for the first time, it prompts for trust and records the decision in local state.

The prompt shows the repo config path and configured hook commands. Declining trust aborts before creating or removing a worktree.

## Registry And State

Sessions stores durable state in `~/.sessions/registry.json`.

All registry writes are protected by a file lock and written atomically. This matters because multiple agents can call `sessions agent set` at the same time.

Trusted repo decisions are stored next to the registry in `~/.sessions/trusted.json`.

## Development

Run the test suite:

```sh
make test
```

Build without installing:

```sh
make build
```

Run directly from source:

```sh
go run ./cmd/sessions --version
go run ./cmd/sessions help
```

Manual validation notes are in [docs/validation.md](docs/validation.md).

None of that belongs in the core until the boring workflow proves itself.

## License

MIT. See [LICENSE](LICENSE).
