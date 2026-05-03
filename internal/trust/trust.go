package trust

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	Path string
}

type File struct {
	Repos map[string]RepoTrust `json:"repos"`
}

type RepoTrust struct {
	ID        string    `json:"id"`
	CommonDir string    `json:"commonDir"`
	Path      string    `json:"path"`
	TrustedAt time.Time `json:"trustedAt"`
}

func NewStore(path string) Store {
	return Store{Path: path}
}

func (s Store) IsTrusted(repoID string) (bool, error) {
	file, err := s.read()
	if err != nil {
		return false, err
	}
	_, ok := file.Repos[repoID]
	return ok, nil
}

func (s Store) Trust(repoID, commonDir, path string) error {
	file, err := s.read()
	if err != nil {
		return err
	}
	if file.Repos == nil {
		file.Repos = map[string]RepoTrust{}
	}
	file.Repos[repoID] = RepoTrust{
		ID:        repoID,
		CommonDir: commonDir,
		Path:      path,
		TrustedAt: time.Now().UTC(),
	}
	return s.write(file)
}

func (s Store) Require(repoID, commonDir, repoConfigPath, onCreate, onRemove string, stdin io.Reader, stderr io.Writer) error {
	trusted, err := s.IsTrusted(repoID)
	if err != nil {
		return err
	}
	if trusted {
		return nil
	}

	fmt.Fprintf(stderr, "Sessions repo hooks are configured in %s\n", repoConfigPath)
	if onCreate != "" {
		fmt.Fprintf(stderr, "  on_create: %s\n", onCreate)
	}
	if onRemove != "" {
		fmt.Fprintf(stderr, "  on_remove: %s\n", onRemove)
	}
	fmt.Fprintln(stderr, "These commands execute repo-controlled code.")
	fmt.Fprint(stderr, "Trust this repo for Sessions hooks? [y/N] ")

	reader := bufio.NewReader(stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read trust prompt response: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return errors.New("repo hooks are not trusted")
	}
	return s.Trust(repoID, commonDir, repoConfigPath)
}

func (s Store) read() (File, error) {
	var file File
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			file.Repos = map[string]RepoTrust{}
			return file, nil
		}
		return file, fmt.Errorf("read trusted repos %s: %w", s.Path, err)
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return file, fmt.Errorf("parse trusted repos %s: %w", s.Path, err)
	}
	if file.Repos == nil {
		file.Repos = map[string]RepoTrust{}
	}
	return file, nil
}

func (s Store) write(file File) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create trusted repo dir: %w", err)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode trusted repos: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.Path, data, 0o600); err != nil {
		return fmt.Errorf("write trusted repos %s: %w", s.Path, err)
	}
	return nil
}
