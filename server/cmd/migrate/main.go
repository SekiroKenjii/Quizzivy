// Command migrate applies the SQL migrations.
//
// This exists instead of shipping the goose CLI, which links drivers for MySQL,
// SQLite, Turso, Vertica, YDB and several others -- a dozen dependencies this
// project does not use, in a binary that runs against a public deployment. The
// goose LIBRARY is already a dependency, so this is the same migration engine
// at the same version with only the Postgres driver attached.
//
// Run by Fly's release_command before a new version takes traffic, using the
// quizzivy_migrate role. The API itself connects as quizzivy_app and cannot run
// DDL (§13.5), so this is the only path by which the schema changes.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	dir := flag.String("dir", "/app/migrations", "migrations directory")
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		command = "up"
	}

	dsn := os.Getenv("MIGRATE_DATABASE_URL")
	if dsn == "" {
		log.Fatal("MIGRATE_DATABASE_URL is required (the quizzivy_migrate role, not quizzivy_app)")
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Same reason as the API: Neon may be resuming a suspended compute, and the
	// release command is often the very first connection after a quiet period.
	if err := waitReady(conn, 60*time.Second); err != nil {
		log.Fatalf("connect: %v", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("dialect: %v", err)
	}

	if err := run(conn, *dir, command); err != nil {
		log.Fatalf("%s: %v", command, err)
	}
}

func waitReady(conn *sql.DB, budget time.Duration) error {
	deadline := time.Now().Add(budget)
	var lastErr error
	for attempt := 1; ; attempt++ {
		if err := conn.Ping(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("not ready after %s (%d attempts): %w", budget, attempt, lastErr)
		}
		time.Sleep(time.Second)
	}
}

func run(conn *sql.DB, dir, command string) error {
	switch command {
	case "up":
		return goose.Up(conn, dir)
	case "down":
		return goose.Down(conn, dir)
	case "status":
		return goose.Status(conn, dir)
	case "version":
		return goose.Version(conn, dir)
	case "reset":
		// Destructive. Deliberately not reachable from the release command;
		// available for local and CI use only.
		return goose.Reset(conn, dir)
	default:
		return fmt.Errorf("unknown command %q (up, down, status, version, reset)", command)
	}
}
