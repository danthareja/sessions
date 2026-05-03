package editor

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"sessions/internal/config"
)

type Resolved struct {
	Name    string
	Command string
	Args    []string
}

func Close(editors map[string]config.EditorConfig, name, path string, stderr io.Writer) (bool, error) {
	resolved, err := ResolveClose(editors, name, path)
	if err != nil {
		return false, err
	}
	if resolved.Command == "" {
		return false, nil
	}
	if _, err := exec.LookPath(resolved.Command); err != nil {
		fmt.Fprintf(stderr, "warning: editor %q close command %q is not on PATH; editor window was not closed\n", resolved.Name, resolved.Command)
		return true, nil
	}
	if err := Start(resolved); err != nil {
		fmt.Fprintf(stderr, "warning: could not close editor %q: %v\n", resolved.Name, err)
		return true, nil
	}
	return true, nil
}

func Launch(editors map[string]config.EditorConfig, name, path string, stderr io.Writer) error {
	resolved, err := Resolve(editors, name, path)
	if err != nil {
		return err
	}
	if resolved.Command == "" {
		return nil
	}
	if _, err := exec.LookPath(resolved.Command); err != nil {
		fmt.Fprintf(stderr, "warning: editor %q command %q is not on PATH; session was created but editor was not opened\n", resolved.Name, resolved.Command)
		return nil
	}
	if err := Start(resolved); err != nil {
		fmt.Fprintf(stderr, "warning: could not launch editor %q: %v\n", resolved.Name, err)
		return nil
	}
	return nil
}

func Resolve(editors map[string]config.EditorConfig, name, path string) (Resolved, error) {
	if name == "" || name == "none" {
		return Resolved{Name: name}, nil
	}
	cfg, ok := editors[name]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown editor %q", name)
	}
	if cfg.Command == "" {
		return Resolved{}, fmt.Errorf("editor %q has no command configured", name)
	}
	args := make([]string, 0, len(cfg.Args))
	for _, arg := range cfg.Args {
		args = append(args, strings.ReplaceAll(arg, "{path}", path))
	}
	return Resolved{Name: name, Command: cfg.Command, Args: args}, nil
}

func ResolveClose(editors map[string]config.EditorConfig, name, path string) (Resolved, error) {
	if name == "" || name == "none" {
		return Resolved{Name: name}, nil
	}
	cfg, ok := editors[name]
	if !ok {
		return Resolved{}, fmt.Errorf("unknown editor %q", name)
	}
	if cfg.CloseCommand == "" {
		return Resolved{Name: name}, nil
	}
	args := make([]string, 0, len(cfg.CloseArgs))
	for _, arg := range cfg.CloseArgs {
		args = append(args, strings.ReplaceAll(arg, "{path}", path))
	}
	return Resolved{Name: name, Command: cfg.CloseCommand, Args: args}, nil
}

func Open(resolved Resolved) error {
	if resolved.Command == "" {
		return nil
	}
	if _, err := exec.LookPath(resolved.Command); err != nil {
		return fmt.Errorf("editor %q command %q is not on PATH", resolved.Name, resolved.Command)
	}
	return Start(resolved)
}

func Start(resolved Resolved) error {
	cmd := exec.Command(resolved.Command, resolved.Args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch editor %q: %w", resolved.Name, err)
	}
	return nil
}

func (r Resolved) CommandLine() string {
	if r.Command == "" {
		return ""
	}
	parts := append([]string{r.Command}, r.Args...)
	for i, part := range parts {
		parts[i] = shellQuote(part)
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
