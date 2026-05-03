package editor

import (
	"reflect"
	"testing"

	"sessions/internal/config"
)

func TestResolvedCommandLineUsesShellSafeQuoting(t *testing.T) {
	resolved := Resolved{
		Command: "code",
		Args:    []string{"--new-window", "/tmp/a dir/$(touch pwned)`x`/it's-here"},
	}
	got := resolved.CommandLine()
	want := `'code' '--new-window' '/tmp/a dir/$(touch pwned)` + "`x`" + `/it'"'"'s-here'`
	if got != want {
		t.Fatalf("CommandLine() = %q, want %q", got, want)
	}
}

func TestResolveCloseUsesCloseCommand(t *testing.T) {
	resolved, err := ResolveClose(map[string]config.EditorConfig{
		"cursor": {
			Command:      "cursor",
			Args:         []string{"--new-window", "{path}"},
			CloseCommand: "close-cursor",
			CloseArgs:    []string{"--folder", "{path}"},
		},
	}, "cursor", "/tmp/work")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Command != "close-cursor" || !reflect.DeepEqual(resolved.Args, []string{"--folder", "/tmp/work"}) {
		t.Fatalf("unexpected close resolution: %+v", resolved)
	}
}

func TestResolveCloseNoopsWithoutCloseCommand(t *testing.T) {
	resolved, err := ResolveClose(map[string]config.EditorConfig{
		"cursor": {Command: "cursor", Args: []string{"{path}"}},
	}, "cursor", "/tmp/work")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Command != "" || len(resolved.Args) != 0 {
		t.Fatalf("expected noop close resolution, got %+v", resolved)
	}
}
