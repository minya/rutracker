package main

import (
	"os"
	"testing"
)

func withStdin(t *testing.T, content string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Seek(0, 0)
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = old; f.Close() })
}

// F59: the password comes from the environment or stdin, not argv.
func TestReadPasswordSources(t *testing.T) {
	t.Setenv(passwordEnv, "from-env")
	withStdin(t, "from-stdin\n")
	if p, err := readPassword(""); err != nil || p != "from-env" {
		t.Fatalf("env: %q, %v", p, err)
	}

	t.Setenv(passwordEnv, "")
	if p, err := readPassword(""); err != nil || p != "from-stdin" {
		t.Fatalf("stdin: %q, %v", p, err)
	}

	withStdin(t, "")
	if _, err := readPassword(""); err == nil {
		t.Fatal("no env and empty stdin: want an error")
	}

	if p, err := readPassword("from-flag"); err != nil || p != "from-flag" {
		t.Fatalf("deprecated flag: %q, %v", p, err)
	}
}
