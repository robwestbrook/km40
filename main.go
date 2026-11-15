package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"km40/internal/api"
	"km40/internal/ingest"
	"km40/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "km40.db", "SQLite database file")
	noIngest := flag.Bool("no-ingest", false, "do not backfill or poll; serve existing data only")
	reparse := flag.Bool("reparse", false, "re-decode every stored report from its raw text before serving")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer st.Close()

	static, err := fs.Sub(webBuild, "web/build")
	if err != nil {
		log.Fatalf("embedded web build: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *reparse {
		n, err := st.Reparse(ctx)
		if err != nil {
			log.Fatalf("reparse: %v", err)
		}
		log.Printf("reparsed %d observations", n)
	}

	// Records come from the last snapshot so the first page load has them; a
	// fresh rebuild runs in the background (it is a full-table aggregate).
	if err := st.LoadRecords(ctx); err != nil {
		log.Printf("records snapshot: %v", err)
	}
	go func() {
		start := time.Now()
		if err := st.RefreshRecords(ctx); err != nil && ctx.Err() == nil {
			log.Printf("records: %v", err)
		} else if ctx.Err() == nil {
			log.Printf("records refreshed in %s", time.Since(start).Round(time.Millisecond))
		}
	}()

	if !*noIngest {
		client := &http.Client{Timeout: 30 * time.Second}
		go func() {
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
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()

	log.Printf("KM40 dashboard on http://localhost%s (database %s)", *addr, *dbPath)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
