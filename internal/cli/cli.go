package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"sessions/internal/commands"
)

const Version = "0.0.0-dev"

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	if args[0] == "--version" || args[0] == "version" {
		fmt.Fprintln(stdout, Version)
		return 0
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "sessions: determine cwd: %v\n", err)
		return 1
	}

	env := commands.Env{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Cwd:    cwd,
		Getenv: os.Getenv,
	}

	ctx := context.Background()
	var runErr error
	switch args[0] {
	case "init":
		runErr = commands.Init(ctx, env, args[1:])
	case "new":
		runErr = commands.New(ctx, env, args[1:])
	case "ls":
		runErr = commands.List(ctx, env, args[1:])
	case "agent":
		runErr = commands.Agent(ctx, env, args[1:])
	case "focus":
		runErr = commands.Focus(ctx, env, args[1:])
	case "remove", "rm":
		runErr = commands.Remove(ctx, env, args[1:])
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "sessions: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}

	if runErr != nil {
		fmt.Fprintf(stderr, "sessions: %v\n", runErr)
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  sessions init
  sessions new <name> [--base <ref>] [--branch <branch>] [--editor code|cursor|zed|none] [--no-setup]
  sessions ls [--json]
  sessions agent set <agent> --state running|needs-input|ready|failed|idle [--message <text>] [--session <session>]
  sessions agent attach <agent> --provider code|cursor|zed|custom --target <id> [--session <session>] [--pid <pid>] [--ttl <duration>] [--metadata <json>]
  sessions agent show <agent> [--session <session>] [--json]
  sessions focus <session> [--agent <agent>]
  sessions focus --next [--all]
  sessions rm [<name>] [--force]
  sessions remove [<name>] [--force]
  sessions --version`)
}
