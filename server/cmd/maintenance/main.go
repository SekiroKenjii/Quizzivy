package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

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
	command, err := maintenance.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	dsn := os.Getenv("MAINTENANCE_DATABASE_URL")
	if dsn == "" {
		return errors.New("MAINTENANCE_DATABASE_URL is required; use the privileged maintenance/owner role")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return errors.New("invalid maintenance database configuration")
	}
	defer pool.Close()
	report, err := command.Execute(ctx, pool)
	if err != nil {
		return fmt.Errorf("maintenance failed: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
