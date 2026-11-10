// Command tokenledger is the local-first token ledger for Claude Code and
// Codex: per-turn fresh vs cached input, output, and API-equivalent cost.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/config"
	"github.com/BlasterSystems/token-literacy/internal/doctor"
	"github.com/BlasterSystems/token-literacy/internal/ingest/codex"
	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
	"github.com/BlasterSystems/token-literacy/internal/server"
	"github.com/BlasterSystems/token-literacy/internal/simulate"
)

var version = "0.1.0-dev"

const defaultAddr = "127.0.0.1:4318"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println("tokenledger " + version)
	case "serve", "up":
		err = cmdServe(os.Args[2:])
	case "setup":
		err = cmdSetup(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "simulate":
		err = cmdSimulate(os.Args[2:])
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: tokenledger <command>

commands:
  serve     start the local collector and dashboard (alias: up)
  setup     merge the Claude Code OTel env block into ~/.claude/settings.json
  doctor    explain the current installation state
  simulate  load fixture files through the production ingest path
  version   print the version
  help      print this message
`)
}

func defaultDBPath() string {
	if p := os.Getenv("TOKENLEDGER_DB"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".tokenledger", "ledger.db")
	}
	return filepath.Join(home, ".tokenledger", "ledger.db")
}

func overridePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tokenledger", "pricing.json")
}

// running is a started collector; stop shuts it down.
type running struct {
	httpSrv      *http.Server
	db           *ledger.DB
	cancelPoller context.CancelFunc
	Addr         string
}

// startServe opens the ledger, starts the Codex tailer, and begins serving
// on 127.0.0.1 (port 0 picks a free port). The caller owns stop().
func startServe(port int, dbPath, codexDir string, noCodex bool) (*running, error) {
	db, err := ledger.Open(dbPath)
	if err != nil {
		return nil, err
	}
	r := &running{db: db}

	override, err := pricing.LoadOverride(overridePath())
	if err != nil {
		db.Close()
		return nil, err
	}

	if !noCodex {
		dir := codexDir
		if dir == "" {
			dir, _ = codex.SessionsDir()
		}
		if dir != "" {
			poller := &codex.Poller{DB: db, Dir: dir, Override: override, Logf: log.Printf, Warnf: log.Printf}
			var pollerCtx context.Context
			pollerCtx, r.cancelPoller = context.WithCancel(context.Background())
			go poller.Run(pollerCtx)
			log.Printf("codex tailer watching %s", dir)
		}
	}

	handler := server.New(server.Info{
		Version: version,
		Started: time.Now(),
		DBPath:  dbPath,
		CodexDir: func() string {
			if noCodex {
				return ""
			}
			dir := codexDir
			if dir == "" {
				dir, _ = codex.SessionsDir()
			}
			return dir
		}(),
		CodexDirExists: codexDirExists(noCodex, codexDir),
		WSL:            detectWSL(),
	}, db, override)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		r.stop()
		return nil, err
	}
	r.Addr = ln.Addr().String()
	r.httpSrv = &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = r.httpSrv.Serve(ln) }()
	log.Printf("tokenledger %s listening on http://%s (db: %s)", version, r.Addr, dbPath)
	return r, nil
}

// stop shuts the collector down.
func (r *running) stop() error {
	if r.cancelPoller != nil {
		r.cancelPoller()
	}
	var err error
	if r.httpSrv != nil {
		err = r.httpSrv.Close()
	}
	if r.db != nil {
		if dbErr := r.db.Close(); dbErr != nil && err == nil {
			err = dbErr
		}
	}
	return err
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 4318, "port to bind on 127.0.0.1")
	dbPath := fs.String("db", defaultDBPath(), "ledger database path")
	codexDir := fs.String("codex-dir", "", "Codex sessions directory (default: $CODEX_HOME/sessions or ~/.codex/sessions)")
	noCodex := fs.Bool("no-codex", false, "disable the Codex rollout poller")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if env := os.Getenv("TOKENLEDGER_ADDR"); env != "" && *port == 4318 {
		if p, err := parsePort(env); err == nil {
			*port = p
		}
	}

	r, err := startServe(*port, *dbPath, *codexDir, *noCodex)
	if err != nil {
		return err
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	return r.stop()
}

func parsePort(addr string) (int, error) {
	var port int
	_, err := fmt.Sscanf(addr, "127.0.0.1:%d", &port)
	return port, err
}

// codexDirExists resolves and stats the effective Codex sessions directory.
func codexDirExists(noCodex bool, codexDir string) bool {
	if noCodex {
		return false
	}
	if codexDir == "" {
		var ok bool
		codexDir, ok = codex.SessionsDir()
		if !ok {
			return false
		}
	}
	fi, err := os.Stat(codexDir)
	return err == nil && fi.IsDir()
}

// detectWSL reports a Microsoft WSL kernel (dashboard loopback caveat).
func detectWSL() bool {
	data, err := os.ReadFile("/proc/version")
	return err == nil && bytes.Contains(bytes.ToLower(data), []byte("microsoft"))
}

func cmdSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	force := fs.Bool("force", false, "rewrite the env block even when unchanged (and prune stale marker keys)")
	endpoint := fs.String("endpoint", "http://"+defaultAddr, "collector OTLP endpoint to configure")
	if err := fs.Parse(args); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := config.ClaudeSettingsPath(home, "")
	res, err := config.ApplyClaudeSetup(path, config.ClaudeEnvBlock(*endpoint), config.Options{
		DryRun: *dryRun,
		Force:  *force,
	})
	if err != nil {
		return err
	}

	out := os.Stdout
	if res.DryRun {
		fmt.Fprintf(out, "dry run for %s:\n", res.Path)
	} else {
		fmt.Fprintf(out, "wrote %s\n", res.Path)
	}
	if res.Backup != "" {
		fmt.Fprintf(out, "backup: %s\n", res.Backup)
	}
	if len(res.Diff) == 0 {
		fmt.Fprintln(out, "no changes needed")
		return nil
	}
	for _, line := range res.Diff {
		fmt.Fprintf(out, "  %s\n", line)
	}
	if res.DryRun && res.Changed {
		fmt.Fprintln(out, "(dry run: nothing written)")
	}
	return nil
}

func cmdDoctor(args []string) error {
	report, err := runDoctor(args)
	if err != nil {
		return err
	}
	report.Render(os.Stdout)
	if report.Failures() > 0 {
		os.Exit(1)
	}
	return nil
}

func cmdSimulate(args []string) error {
	fs := flag.NewFlagSet("simulate", flag.ContinueOnError)
	dir := fs.String("fixture", "", "directory of fixture files (.json/.json.gz OTLP, .jsonl rollouts)")
	dbPath := fs.String("db", defaultDBPath(), "ledger database path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" {
		return fmt.Errorf("simulate requires --fixture DIR")
	}
	db, err := ledger.Open(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	override, err := pricing.LoadOverride(overridePath())
	if err != nil {
		return err
	}
	inserted, err := simulate.Run(db, *dir, override)
	if err != nil {
		return err
	}
	fmt.Printf("simulated %s: %d rows inserted\n", *dir, inserted)
	return nil
}

func runDoctor(args []string) (doctor.Report, error) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDBPath(), "ledger database path")
	if err := fs.Parse(args); err != nil {
		return doctor.Report{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return doctor.Report{}, err
	}
	return doctor.Run(doctor.Inputs{
		DBPath:         *dbPath,
		ClaudeSettings: config.ClaudeSettingsPath(home, ""),
		ExpectedClaude: config.ClaudeEnvBlock("http://" + defaultAddr),
		CollectorURL:   "http://" + defaultAddr + "/health",
		OverridePath:   overridePath(),
		HTTPClient:     &http.Client{Timeout: 3 * time.Second},
	}), nil
}
