// Package log provides a TDLib-compatible logging layer: stderr output with
// ANSI colors (on TTY), verbosity levels, and the official timestamp format.
// Replaces stdlib "log" across internal/ so console output matches the
// official telegram-bot-api server. See specs/5-console-log-parity.
package log

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Level is a TDLib-style log severity (FATAL=0 … DEBUG=4).
type Level int

const (
	LevelFatal   Level = 0
	LevelError   Level = 1
	LevelWarning Level = 2
	LevelInfo    Level = 3
	LevelDebug   Level = 4
)

var (
	mu        sync.Mutex
	verbosity = LevelError // default 1 (ERROR); matches telegram-bot-api default
	useColor  bool
)

func init() {
	useColor = isTerminal()
}

// SetVerbosity sets the maximum level emitted (0=FATAL … 4=DEBUG).
func SetVerbosity(level int) {
	mu.Lock()
	defer mu.Unlock()
	if level < 0 {
		level = 0
	}
	if level > int(LevelDebug) {
		level = int(LevelDebug)
	}
	verbosity = Level(level)
}

// Fatal logs a FATAL message and exits with code 1.
func Fatal(msg string) {
	emit(LevelFatal, msg, false)
	os.Exit(1)
}

// Error logs at ERROR level.
func Error(format string, args ...any) { emit(LevelError, fmt.Sprintf(format, args...), false) }

// Warn logs at WARNING level (subject to verbosity).
func Warn(format string, args ...any) { emit(LevelWarning, fmt.Sprintf(format, args...), false) }

// WarnAlways logs at WARNING level regardless of verbosity.
func WarnAlways(format string, args ...any) {
	emit(LevelWarning, fmt.Sprintf(format, args...), true)
}

// Info logs at INFO level.
func Info(format string, args ...any) { emit(LevelInfo, fmt.Sprintf(format, args...), false) }

// InfoAlways logs at INFO level regardless of verbosity.
func InfoAlways(format string, args ...any) {
	emit(LevelInfo, fmt.Sprintf(format, args...), true)
}

// Debug logs at DEBUG level.
func Debug(format string, args ...any) { emit(LevelDebug, fmt.Sprintf(format, args...), false) }

// emit writes one formatted line to stderr if the level passes the verbosity
// gate (or force is true). Color is applied only when stderr is a TTY.
func emit(lvl Level, msg string, force bool) {
	mu.Lock()
	defer mu.Unlock()
	if !force && lvl > verbosity {
		return
	}

	ts := time.Now().Format("2006-01-02 15:04:05.000000")
	line := fmt.Sprintf("[%s] [%s] %s", ts, levelName(lvl), msg)
	if useColor {
		line = colorFor(lvl) + line + "\x1b[0m"
	}
	fmt.Fprintln(os.Stderr, line)
}

func levelName(lvl Level) string {
	switch lvl {
	case LevelFatal:
		return "FATAL"
	case LevelError:
		return "ERROR"
	case LevelWarning:
		return "WARNING"
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	}
	return "?"
}

// colorFor returns the ANSI escape for the level (matching TDLib's
// DefaultLog::do_append): bold red for FATAL/ERROR, bold yellow for WARNING,
// bold cyan for INFO, none for DEBUG.
func colorFor(lvl Level) string {
	switch lvl {
	case LevelFatal, LevelError:
		return "\x1b[1;31m"
	case LevelWarning:
		return "\x1b[1;33m"
	case LevelInfo:
		return "\x1b[1;36m"
	}
	return ""
}

// isTerminal reports whether stderr is a character device (TTY). When false
// (piped to a file/aggregator), no ANSI color codes are emitted.
func isTerminal() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
