// Command mtgo-bot-api is the Telegram Bot API server entry point.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/manager"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/stats"
	"github.com/mtgo-labs/mtgo-bot-api/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("mtgo-bot-api", flag.ContinueOnError)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "Usage: mtgo-bot-api --api-id=<arg> --api-hash=<arg> [--local] [OPTION]...")
		_, _ = fmt.Fprintln(fs.Output(), "Telegram Bot API server")
		fs.PrintDefaults()
	}

	var (
		apiID           int
		apiHash         string
		localMode       bool
		httpPort        int
		httpStatPort    int
		dir             string
		tempDir         string
		filter          string
		maxWebhookConns int
		msgCacheCap     int
		httpIPAddress   string
		proxy           string
		verbosity       int
		showVersion     bool
	)
	fs.IntVar(&apiID, "api-id", envInt("TELEGRAM_API_ID", 0), "application id (or $TELEGRAM_API_ID)")
	fs.StringVar(&apiHash, "api-hash", os.Getenv("TELEGRAM_API_HASH"), "application hash (or $TELEGRAM_API_HASH)")
	fs.BoolVar(&localMode, "local", false, "allow the Bot API server to serve local requests")
	fs.IntVar(&httpPort, "http-port", 8081, "HTTP listening port")
	fs.IntVar(&httpStatPort, "http-stat-port", 0, "HTTP statistics port")
	fs.StringVar(&dir, "dir", ".mtgo-bot-api", "server working directory")
	fs.StringVar(&tempDir, "temp-dir", os.TempDir(), "directory for storing temporary files")
	fs.StringVar(&filter, "filter", "", "\"<remainder>/<modulo>\". Allow only bots with bot_user_id%modulo==remainder")
	fs.IntVar(&maxWebhookConns, "max-webhook-connections", 100, "default max webhook connections per bot")
	fs.IntVar(&msgCacheCap, "message-cache-cap", 10000, "maximum cached messages per bot")
	fs.StringVar(&httpIPAddress, "http-ip-address", "", "local IP address to accept HTTP connections on")
	fs.StringVar(&proxy, "proxy", "", "HTTP proxy for outgoing webhook requests (http://host:port)")
	fs.IntVar(&verbosity, "verbosity", 1, "log verbosity level")
	fs.BoolVar(&showVersion, "version", false, "display version number and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if showVersion {
		fmt.Printf("mtgo-bot-api v%s\n", version.Version)
		fmt.Printf("Bot API %s\n", version.BotAPIVersion)
		return nil
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if apiID <= 0 || apiHash == "" {
		return errors.New("you must provide valid --api-id and --api-hash (obtain at https://my.telegram.org)")
	}
	_ = filter // validated/used when ClientManager token routing is implemented (US2+)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create working dir: %w", err)
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}

	botlog.SetVerbosity(verbosity)

	params := &manager.Parameters{
		APIID:                  int32(apiID),
		APIHash:                apiHash,
		LocalMode:              localMode,
		WorkingDir:             dir,
		TempDir:                tempDir,
		DefaultMaxWebhookConns: maxWebhookConns,
		MsgCacheCap:            msgCacheCap,
	}
	mgr, err := manager.New(params)
	if err != nil {
		return fmt.Errorf("init manager: %w", err)
	}
	defer func() { _ = mgr.Close() }() // fallback if Shutdown isn't reached
	mgr.StartGC()
	st := stats.New()
	srv := server.New(server.Config{
		Addr:    joinHostPort(httpIPAddress, httpPort),
		TempDir: tempDir,
	}, mgr)
	srv.SetStatsHandler(st.Handler())

	// Stats port (separate listener for metrics).
	var statSrv *http.Server
	if httpStatPort > 0 {
		statSrv = &http.Server{
			Addr:    joinHostPort(httpIPAddress, httpStatPort),
			Handler: st.Handler(),
		}
		go func() {
			botlog.Info("stats endpoint on :%d", httpStatPort)
			if err := statSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				botlog.Error("stats server: %v", err)
			}
		}()
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	startTime := time.Now()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	// Lifecycle messages are always emitted, but use INFO severity because they
	// are normal process state changes, not warnings.
	botlog.InfoAlways("Bot API server started")

	sigCh := notifySignals(syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
		uptime := int(time.Since(startTime).Seconds())
		botlog.InfoAlways("Stopping engine with uptime %d seconds by a signal", uptime)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = srv.Shutdown(ctx)
		if statSrv != nil {
			_ = statSrv.Shutdown(ctx)
		}
		if err := mgr.Shutdown(ctx); err != nil {
			botlog.Error("manager shutdown: %v", err)
		}
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
	}
	botlog.InfoAlways("mtgo-bot-api stopped")
	return nil
}

const shutdownTimeout = 15 * time.Second

func joinHostPort(ip string, port int) string {
	if ip == "" {
		return fmt.Sprintf(":%d", port)
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

var notifySignals = func(signals ...os.Signal) <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	return ch
}
