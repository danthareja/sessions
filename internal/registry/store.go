package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Store struct {
	Path string
}

func NewStore(path string) Store {
	return Store{Path: path}
}

func (s Store) Read() (*Registry, error) {
	if s.Path == "" {
		return nil, errors.New("registry path is empty")
	}
	release, err := lockRegistry(s.Path, false)
	if err != nil {
		return nil, err
	}
	defer release()
	return s.readUnlocked()
}

func (s Store) Update(fn func(*Registry) error) error {
	if s.Path == "" {
		return errors.New("registry path is empty")
	}
	release, err := lockRegistry(s.Path, true)
	if err != nil {
		return err
	}
	defer release()

	reg, err := s.readUnlocked()
	if err != nil {
		return err
	}
	if err := fn(reg); err != nil {
		return err
	}
	reg.Ensure()
	if reg.Version > SupportedVersion {
		return higherVersionError(reg.Version)
	}
	return s.writeUnlocked(reg)
}

func (s Store) readUnlocked() (*Registry, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Empty(), nil
		}
		return nil, fmt.Errorf("read registry %s: %w", s.Path, err)
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry %s: %w", s.Path, err)
	}
	reg.Ensure()
	if reg.Version > SupportedVersion {
		return nil, higherVersionError(reg.Version)
	}
	return &reg, nil
}

func (s Store) writeUnlocked(reg *Registry) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create registry dir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".registry-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp registry: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp registry: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp registry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp registry: %w", err)
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("replace registry: %w", err)
	}
	cleanup = false
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("sync registry dir: %w", err)
	}
	return nil
}

func higherVersionError(version int) error {
	return fmt.Errorf("registry version %d is newer than this Sessions binary supports (version %d); upgrade Sessions", version, SupportedVersion)
}
