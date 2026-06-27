package log

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// captureStderr swaps os.Stderr to capture output, runs fn, restores.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	origColor := useColor
	useColor = false // deterministic: no color in tests
	defer func() {
		os.Stderr = orig
		useColor = origColor
	}()
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()
	fn()
	w.Close()
	out := <-done
	return out
}

func TestEmit_Format(t *testing.T) {
	out := captureStderr(t, func() { InfoAlways("Bot API server started") })
	// Must contain [timestamp] [INFO] message.
	if !strings.Contains(out, "[INFO] Bot API server started") {
		t.Errorf("missing level+message in %q", out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("must start with timestamp bracket; got %q", out)
	}
	// Timestamp shape: [YYYY-MM-DD HH:MM:SS.ffffff]
	if !strings.Contains(out, "] [INFO]") {
		t.Errorf("expected '] [INFO]' separator; got %q", out)
	}
}

func TestEmit_NoColorWhenNotTTY(t *testing.T) {
	out := captureStderr(t, func() { WarnAlways("test") })
	if strings.Contains(out, "\x1b[") {
		t.Errorf("ANSI color emitted when not a TTY: %q", out)
	}
}

func TestVerbosity_GatesLevels(t *testing.T) {
	SetVerbosity(1) // ERROR only
	out := captureStderr(t, func() {
		Error("err visible")
		Warn("warn hidden")
		Info("info hidden")
	})
	if !strings.Contains(out, "err visible") {
		t.Error("ERROR should be visible at verbosity 1")
	}
	if strings.Contains(out, "warn hidden") {
		t.Error("WARNING should be hidden at verbosity 1")
	}
	if strings.Contains(out, "info hidden") {
		t.Error("INFO should be hidden at verbosity 1")
	}
}

func TestVerbosity_AllLevelsAt3(t *testing.T) {
	SetVerbosity(3) // INFO
	out := captureStderr(t, func() {
		Error("err")
		Warn("warn")
		Info("info")
		Debug("debug hidden")
	})
	for _, want := range []string{"err", "warn", "info"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q should be visible at verbosity 3; got %q", want, out)
		}
	}
	if strings.Contains(out, "debug hidden") {
		t.Error("DEBUG should be hidden at verbosity 3")
	}
}

func TestInfoAlways_BypassesVerbosity(t *testing.T) {
	SetVerbosity(0) // FATAL only
	out := captureStderr(t, func() { InfoAlways("startup always shown") })
	if !strings.Contains(out, "startup always shown") {
		t.Error("InfoAlways must emit even at verbosity 0")
	}
}

func TestWarn_RespectsVerbosity(t *testing.T) {
	SetVerbosity(0) // FATAL only
	out := captureStderr(t, func() { Warn("regular warn hidden") })
	if strings.Contains(out, "regular warn hidden") {
		t.Error("Warn must respect verbosity 0 (hidden)")
	}
}

func TestSetVerbosity_Clamps(t *testing.T) {
	SetVerbosity(-5)
	if verbosity != LevelFatal {
		t.Error("negative verbosity should clamp to 0")
	}
	SetVerbosity(99)
	if verbosity != LevelDebug {
		t.Error("oversized verbosity should clamp to 4")
	}
}

func TestLevelName(t *testing.T) {
	cases := map[Level]string{
		LevelFatal: "FATAL", LevelError: "ERROR", LevelWarning: "WARNING",
		LevelInfo: "INFO", LevelDebug: "DEBUG",
	}
	for lvl, want := range cases {
		if got := levelName(lvl); got != want {
			t.Errorf("levelName(%d) = %q, want %q", lvl, got, want)
		}
	}
}

func TestColorFor(t *testing.T) {
	if colorFor(LevelError) != "\x1b[1;31m" {
		t.Error("ERROR color must be bold red")
	}
	if colorFor(LevelWarning) != "\x1b[1;33m" {
		t.Error("WARNING color must be bold yellow")
	}
	if colorFor(LevelInfo) != "\x1b[1;36m" {
		t.Error("INFO color must be bold cyan")
	}
	if colorFor(LevelDebug) != "" {
		t.Error("DEBUG must have no color")
	}
}
