package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"km40/internal/api"
	"km40/internal/ingest"
	"km40/internal/store"
)

func main() {
	// run returns instead of calling log.Fatal inside, so its deferred cleanup
	// (stopping background work, closing the database) always runs.
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "km40.db", "SQLite database file")
	noIngest := flag.Bool("no-ingest", false, "do not backfill or poll; serve existing data only")
	reparse := flag.Bool("reparse", false, "re-decode every stored report from its raw text before serving")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	static, err := fs.Sub(webBuild, "web/build")
	if err != nil {
		return fmt.Errorf("embedded web build: %w", err)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(sigCtx)

	// Background work uses the store, so it must finish before the store
	// closes. Deferred calls run last-in first-out: cancel and wait here, then
	// st.Close above.
	var bg sync.WaitGroup
	defer func() {
		cancel()
		bg.Wait()
	}()

	if *reparse {
		n, err := st.Reparse(ctx)
		if err != nil {
			return fmt.Errorf("reparse: %w", err)
		}
		log.Printf("reparsed %d observations", n)
	}

	// Records come from the last snapshot so the first page load has them; a
	// fresh rebuild runs in the background (it is a full-table aggregate).
	if err := st.LoadRecords(ctx); err != nil {
		log.Printf("records snapshot: %v", err)
	}
	bg.Add(1)
	go func() {
		defer bg.Done()
		start := time.Now()
		if err := st.RefreshRecords(ctx); err != nil && ctx.Err() == nil {
			log.Printf("records: %v", err)
		} else if ctx.Err() == nil {
			log.Printf("records refreshed in %s", time.Since(start).Round(time.Millisecond))
		}
	}()

	if !*noIngest {
		client := &http.Client{Timeout: 30 * time.Second}
		bg.Add(1)
		go func() {
			defer bg.Done()
			if err := ingest.Run(ctx, st, client, func(s string) { log.Print(s) }); err != nil && ctx.Err() == nil {
				log.Printf("ingest stopped: %v", err)
			}
		}()
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(st, static),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	log.Printf("KM40 dashboard listening on %s (database %s)", *addr, *dbPath)
	select {
	case err := <-serveErr:
		return err // could not listen; ListenAndServe never returns nil
	case <-ctx.Done():
	}

	// Let in-flight requests finish before the deferred cleanup closes the
	// database underneath them.
	drain, cancelDrain := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDrain()
	if err := srv.Shutdown(drain); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Print("shut down cleanly")
	return nil
}
