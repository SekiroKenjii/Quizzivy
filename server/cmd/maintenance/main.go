package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"quizzivy/internal/core/maintenance"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	apply := flag.Bool("apply", false, "apply changes; omitted means dry-run")
	batch := flag.Int("batch", 1000, "maximum integrity events in one batch (1..10000)")
	student := flag.String("student", "", "student UUID for anonymize-student")
	flag.Parse()
	command := flag.Arg(0)
	if command != "retain-integrity" && command != "anonymize-student" {
		return errors.New("usage: maintenance [-apply] [-batch N] [-student UUID] retain-integrity|anonymize-student")
	}
	dsn := os.Getenv("MAINTENANCE_DATABASE_URL")
	if dsn == "" {
		return errors.New("MAINTENANCE_DATABASE_URL is required; use the privileged maintenance/owner role")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return errors.New("invalid maintenance database configuration")
	}
	defer pool.Close()
	var report any
	if command == "retain-integrity" {
		report, err = maintenance.RetainEvents(ctx, pool, *apply, *batch)
	} else {
		report, err = maintenance.AnonymizeStudent(ctx, pool, *student, *apply)
	}
	if err != nil {
		return fmt.Errorf("maintenance failed: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
