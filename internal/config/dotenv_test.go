package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `
# comment
ADDR=:9090
export API_KEY="secret-key"
QUOTED='hello'
EMPTY=
NOEQ
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// clear and ensure not set
	_ = os.Unsetenv("ADDR")
	_ = os.Unsetenv("API_KEY")
	_ = os.Unsetenv("QUOTED")

	// pre-set should not be overridden
	t.Setenv("ALREADY", "keep")
	// write ALREADY into file via append — use second load path with explicit key in file
	if err := os.WriteFile(path, []byte(content+"\nALREADY=new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != path {
		t.Fatalf("loaded path=%q", loaded)
	}
	if os.Getenv("ADDR") != ":9090" {
		t.Fatalf("ADDR=%q", os.Getenv("ADDR"))
	}
	if os.Getenv("API_KEY") != "secret-key" {
		t.Fatalf("API_KEY=%q", os.Getenv("API_KEY"))
	}
	if os.Getenv("QUOTED") != "hello" {
		t.Fatalf("QUOTED=%q", os.Getenv("QUOTED"))
	}
	if os.Getenv("ALREADY") != "keep" {
		t.Fatalf("ALREADY should not be overridden, got %q", os.Getenv("ALREADY"))
	}
}

func TestLoadDotEnvMissing(t *testing.T) {
	loaded, err := LoadDotEnv(filepath.Join(t.TempDir(), "nope.env"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded != "" {
		t.Fatalf("expected empty, got %q", loaded)
	}
}
