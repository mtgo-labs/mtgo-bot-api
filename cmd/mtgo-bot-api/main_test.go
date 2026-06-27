package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// captureStdout swaps os.Stdout to a pipe, runs fn, restores, and returns the
// captured output. Mirrors the captureStderr helper in internal/log.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	return <-done
}

// clearCreds unsets the env vars that run() reads as flag defaults, so tests
// control exactly which credentials are supplied.
func clearCreds(t *testing.T) {
	t.Helper()
	t.Setenv("TELEGRAM_API_ID", "")
	t.Setenv("TELEGRAM_API_HASH", "")
}

// --- credential validation (api-id/api-hash) ---

func TestRun_MissingAPIID(t *testing.T) {
	clearCreds(t)
	err := run([]string{"--api-hash", "deadbeef"})
	if err == nil {
		t.Fatal("expected error for missing --api-id, got nil")
	}
	if !strings.Contains(err.Error(), "you must provide valid --api-id and --api-hash") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_MissingAPIHash(t *testing.T) {
	clearCreds(t)
	err := run([]string{"--api-id", "12345"})
	if err == nil {
		t.Fatal("expected error for missing --api-hash, got nil")
	}
	if !strings.Contains(err.Error(), "you must provide valid --api-id and --api-hash") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_BothCredentialsMissing(t *testing.T) {
	clearCreds(t)
	err := run(nil)
	if err == nil {
		t.Fatal("expected error for missing both credentials, got nil")
	}
	if !strings.Contains(err.Error(), "you must provide valid --api-id and --api-hash") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_NegativeAPIID(t *testing.T) {
	// apiID <= 0 is rejected identically to a missing api-id.
	clearCreds(t)
	err := run([]string{"--api-id", "-1", "--api-hash", "deadbeef"})
	if err == nil {
		t.Fatal("expected error for negative --api-id, got nil")
	}
	if !strings.Contains(err.Error(), "you must provide valid --api-id and --api-hash") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_ZeroAPIID(t *testing.T) {
	clearCreds(t)
	err := run([]string{"--api-id", "0", "--api-hash", "deadbeef"})
	if err == nil {
		t.Fatal("expected error for zero --api-id, got nil")
	}
	if !strings.Contains(err.Error(), "you must provide valid --api-id and --api-hash") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- version flag ---

func TestRun_Version(t *testing.T) {
	var runErr error
	out := captureStdout(t, func() {
		runErr = run([]string{"--version"})
	})
	if runErr != nil {
		t.Fatalf("expected nil error for --version, got %v", runErr)
	}
	if !strings.Contains(out, "mtgo-bot-api") {
		t.Errorf("version output missing program name: %q", out)
	}
}

// --- flag parsing ---

func TestRun_HelpFlag(t *testing.T) {
	// flag.ContinueOnError prints usage and returns flag.ErrHelp for -h/--help.
	err := run([]string{"--help"})
	if err == nil {
		t.Fatal("expected error for --help, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp, got %v", err)
	}
}

func TestRun_HelpShortFlag(t *testing.T) {
	err := run([]string{"-h"})
	if err == nil {
		t.Fatal("expected error for -h, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp, got %v", err)
	}
}

func TestRun_InvalidFlag(t *testing.T) {
	err := run([]string{"--no-such-flag"})
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Errorf("unknown flag should not return ErrHelp: %v", err)
	}
}

func TestRun_InvalidFlagValue(t *testing.T) {
	// --http-port expects an int; a non-integer value is a parse error.
	err := run([]string{"--http-port", "not-a-number"})
	if err == nil {
		t.Fatal("expected error for non-integer flag value, got nil")
	}
}

// --- positional args ---

func TestRun_UnexpectedPositionalArgs(t *testing.T) {
	// Valid credentials ensure the positional-arg check (which runs first) is
	// the only failure.
	t.Setenv("TELEGRAM_API_ID", "12345")
	t.Setenv("TELEGRAM_API_HASH", "deadbeef")
	err := run([]string{"extra-arg"})
	if err == nil {
		t.Fatal("expected error for unexpected positional args, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_MultiplePositionalArgs(t *testing.T) {
	t.Setenv("TELEGRAM_API_ID", "12345")
	t.Setenv("TELEGRAM_API_HASH", "deadbeef")
	err := run([]string{"foo", "bar"})
	if err == nil {
		t.Fatal("expected error for multiple positional args, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- directory creation ---

func TestRun_WorkingDirCreationFails(t *testing.T) {
	// A path whose parent component is a regular file → MkdirAll fails.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("TELEGRAM_API_ID", "12345")
	t.Setenv("TELEGRAM_API_HASH", "deadbeef")
	err := run([]string{"--dir", filepath.Join(blocker, "sub")})
	if err == nil {
		t.Fatal("expected error when working dir cannot be created")
	}
	if !strings.Contains(err.Error(), "create working dir") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_TempDirCreationFails(t *testing.T) {
	// --dir is a valid existing temp dir (no side effect); --temp-dir's parent
	// is a regular file so its MkdirAll fails.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Setenv("TELEGRAM_API_ID", "12345")
	t.Setenv("TELEGRAM_API_HASH", "deadbeef")
	err := run([]string{"--dir", t.TempDir(), "--temp-dir", filepath.Join(blocker, "sub")})
	if err == nil {
		t.Fatal("expected error when temp dir cannot be created")
	}
	if !strings.Contains(err.Error(), "create temp dir") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- manager init failure ---

func TestRun_ManagerInitFails(t *testing.T) {
	// A read-only working dir: MkdirAll succeeds (dir exists), but the SQLite
	// store cannot create tqueue.db → manager.New fails before any goroutine
	// or server is started.
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) // restore for t.TempDir cleanup

	// Guard against environments that don't enforce directory permissions.
	if f, err := os.CreateTemp(dir, "probe"); err == nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		t.Skip("filesystem does not enforce read-only dirs; cannot guarantee manager init failure")
	}

	t.Setenv("TELEGRAM_API_ID", "12345")
	t.Setenv("TELEGRAM_API_HASH", "deadbeef")
	err := run([]string{"--dir", dir})
	if err == nil {
		t.Fatal("expected manager init failure on read-only working dir, got nil")
	}
	if !strings.Contains(err.Error(), "init manager") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_GracefulSignalShutdown(t *testing.T) {
	t.Setenv("TELEGRAM_API_ID", "12345")
	t.Setenv("TELEGRAM_API_HASH", "deadbeef")
	origNotifySignals := notifySignals
	sigCh := make(chan os.Signal, 1)
	notifySignals = func(signals ...os.Signal) <-chan os.Signal {
		go func() {
			time.Sleep(10 * time.Millisecond)
			sigCh <- syscall.SIGTERM
		}()
		return sigCh
	}
	t.Cleanup(func() { notifySignals = origNotifySignals })

	err := run([]string{
		"--dir", t.TempDir(),
		"--temp-dir", t.TempDir(),
		"--http-port", "0",
		"--http-stat-port", "0",
	})
	if err != nil {
		t.Fatalf("run graceful shutdown returned %v", err)
	}
}

// --- helpers ---

func TestJoinHostPort(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		port int
		want string
	}{
		{"empty ip binds all", "", 8081, ":8081"},
		{"specific ip", "127.0.0.1", 8081, "127.0.0.1:8081"},
		{"wildcard", "0.0.0.0", 443, "0.0.0.0:443"},
		{"ipv6", "::1", 8081, "::1:8081"},
		{"high port", "", 65535, ":65535"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinHostPort(tt.ip, tt.port); got != tt.want {
				t.Errorf("joinHostPort(%q, %d) = %q, want %q", tt.ip, tt.port, got, tt.want)
			}
		})
	}
}

func TestEnvInt(t *testing.T) {
	t.Run("unset returns default", func(t *testing.T) {
		t.Setenv("MTBA_TEST_ENV_INT", "")
		if got := envInt("MTBA_TEST_ENV_INT", 42); got != 42 {
			t.Errorf("got %d, want 42", got)
		}
	})
	t.Run("set returns value", func(t *testing.T) {
		t.Setenv("MTBA_TEST_ENV_INT", "99")
		if got := envInt("MTBA_TEST_ENV_INT", 42); got != 99 {
			t.Errorf("got %d, want 99", got)
		}
	})
	t.Run("invalid returns default", func(t *testing.T) {
		t.Setenv("MTBA_TEST_ENV_INT", "not-a-number")
		if got := envInt("MTBA_TEST_ENV_INT", 42); got != 42 {
			t.Errorf("got %d, want 42", got)
		}
	})
	t.Run("negative value", func(t *testing.T) {
		t.Setenv("MTBA_TEST_ENV_INT", "-7")
		if got := envInt("MTBA_TEST_ENV_INT", 42); got != -7 {
			t.Errorf("got %d, want -7", got)
		}
	})
}

// --- shutdownTimeout constant ---

func TestShutdownTimeout(t *testing.T) {
	if shutdownTimeout != 15*time.Second {
		t.Errorf("shutdownTimeout = %v, want 15s", shutdownTimeout)
	}
}
