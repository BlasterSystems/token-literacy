package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/BlasterSystems/token-literacy/internal/server"
)

var version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println("tokenledger " + version)
	case "serve", "up":
		if err := serve(); err != nil {
			log.Fatal(err)
		}
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: tokenledger <command>

commands:
  serve    start the local collector and dashboard
  version  print the version
  help     print this message

The collector interface (setup, doctor, export, install) is planned in docs/PLAN.md.
`)
}

func serve() error {
	addr := os.Getenv("TOKENLEDGER_ADDR")
	if addr == "" {
		addr = "127.0.0.1:4318"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	handler := server.NewHandler(server.Info{
		Version: version,
		Started: time.Now(),
		DBPath:  filepath.Join(home, ".tokenledger", "ledger.db"),
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("tokenledger %s listening on http://%s", version, addr)
		errCh <- srv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-sig:
		return srv.Close()
	}
}
