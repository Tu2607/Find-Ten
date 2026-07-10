package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"find-ten-game/internal/api"
	"find-ten-game/internal/leaderboard"
	"find-ten-game/internal/player"
	"find-ten-game/internal/sqlitedb"
)

const shutdownTimeout = 5 * time.Second
const databaseInitTimeout = 5 * time.Second

func main() {
	addr := flag.String("addr", ":8080", "server listen address")
	dbPath := flag.String("db-path", "./data/find-ten.db", "SQLite database path")
	flag.Parse()

	dbCtx, dbCancel := context.WithTimeout(context.Background(), databaseInitTimeout)
	defer dbCancel()

	db, err := sqlitedb.Open(dbCtx, *dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database error: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "database close error: %v\n", err)
		}
	}()

	// Step 35B initializes the player schema here. Later account API work will
	// keep and pass this store as a server dependency.
	if _, err := player.NewStore(dbCtx, db); err != nil {
		fmt.Fprintf(os.Stderr, "player store error: %v\n", err)
		os.Exit(1)
	}

	leaderboardStore, err := leaderboard.NewStore(dbCtx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "leaderboard store error: %v\n", err)
		os.Exit(1)
	}

	server := api.NewServer(leaderboardStore)
	fmt.Fprintf(os.Stdout, "listening on %s\n", *addr)
	if err := listenAndServe(*addr, server); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func listenAndServe(addr string, handler http.Handler) error {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	serverErrors := make(chan error, 1)
	go func() {
		defer close(serverErrors)

		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErrors <- err
	}()

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdownSignals)

	select {
	case err := <-serverErrors:
		return err
	case <-shutdownSignals:
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}

		return <-serverErrors
	}
}
