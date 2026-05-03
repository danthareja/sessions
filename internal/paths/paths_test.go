package paths

import (
	"path/filepath"
	"testing"
)

func TestConfigDirDefaultsToHomeDotSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".sessions")
	if got != want {
		t.Fatalf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestPathsUseHomeConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".sessions")

	configPath, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if configPath != filepath.Join(dir, "config.toml") {
		t.Fatalf("ConfigPath() = %q", configPath)
	}

	registryPath, err := RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	if registryPath != filepath.Join(dir, "registry.json") {
		t.Fatalf("RegistryPath() = %q", registryPath)
	}

	trustedPath, err := TrustedPath()
	if err != nil {
		t.Fatal(err)
	}
	if trustedPath != filepath.Join(dir, "trusted.json") {
		t.Fatalf("TrustedPath() = %q", trustedPath)
	}
}

func TestPathEnvVarsDoNotOverrideHomeConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SESSIONS_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("SESSIONS_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	dir := filepath.Join(home, ".sessions")

	configPath, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if configPath != filepath.Join(dir, "config.toml") {
		t.Fatalf("ConfigPath() = %q", configPath)
	}

	registryPath, err := RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	if registryPath != filepath.Join(dir, "registry.json") {
		t.Fatalf("RegistryPath() = %q", registryPath)
	}
}
