// Command tokenledger is the local-first token ledger for Claude Code and
// Codex: per-turn fresh vs cached input, output, and API-equivalent cost.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/config"
	"github.com/BlasterSystems/token-literacy/internal/doctor"
	"github.com/BlasterSystems/token-literacy/internal/ledger"
	"github.com/BlasterSystems/token-literacy/internal/pricing"
	"github.com/BlasterSystems/token-literacy/internal/server"
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
  serve    start the local collector and dashboard (alias: up)
  setup    merge the Claude Code OTel env block into ~/.claude/settings.json
  doctor   explain the current installation state
  version  print the version
  help     print this message
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

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	port := fs.Int("port", 0, "port to bind on 127.0.0.1")
	dbPath := fs.String("db", defaultDBPath(), "ledger database path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	addr := os.Getenv("TOKENLEDGER_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	if *port != 0 {
		addr = fmt.Sprintf("127.0.0.1:%d", *port)
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

	handler := server.New(server.Info{
		Version: version,
		Started: time.Now(),
		DBPath:  *dbPath,
	}, db, override)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("tokenledger %s listening on http://%s (db: %s)", version, addr, *dbPath)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-sig:
		return srv.Close()
	}
}

func overridePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".tokenledger", "pricing.json")
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
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDBPath(), "ledger database path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	report := doctor.Run(doctor.Inputs{
		DBPath:          *dbPath,
		ClaudeSettings:  config.ClaudeSettingsPath(home, ""),
		ExpectedClaude:  config.ClaudeEnvBlock("http://" + defaultAddr),
		CollectorURL:    "http://" + defaultAddr + "/health",
		OverridePath:    overridePath(),
		HTTPClient:      &http.Client{Timeout: 3 * time.Second},
	})
	report.Render(os.Stdout)
	if report.Failures() > 0 {
		os.Exit(1)
	}
	return nil
}
