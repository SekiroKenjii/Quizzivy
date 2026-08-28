// Command seedadmin prints a temporary teacher credential for bootstrapping a
// fresh deployment.
//
// A new database has no accounts, and seed/01-dev.sql must never be used
// against one: its password is committed to this repository. This generates a
// random one instead and hashes it with the SAME code the server verifies with,
// so the parameters cannot drift.
//
// It deliberately does NOT touch the database. It prints; a human decides where
// the hash goes. The account it is meant for is created with
// `must_change_password = true`, so the printed password survives exactly one
// sign-in before §5.4's forced change screen replaces it.
//
// The PASSWORD line goes to stdout. Do NOT pipe this into `tee`, a log, or a CI
// job that archives output: that writes an admin credential to a file, and the
// must_change_password window is short but it is not zero. Read it, use it,
// close the terminal.
//
//	go run ./cmd/seedadmin
//
//	# NEON_MIGRATE_URL owns the schema; see .env.example. It must be SET --
//	# psql reads an empty conninfo as "use every default", which on a developer
//	# machine means a local socket and a local database, so the INSERT lands
//	# somewhere that is not the deployment being bootstrapped.
//	psql "$NEON_MIGRATE_URL" -v hash="<HASH>" <<'SQL'
//	  INSERT INTO app.users (email, full_name, role, password_hash, must_change_password)
//	  VALUES ('teacher@example.com', 'Name', 'admin', :'hash', true);
//	SQL
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"quizzivy/internal/auth"
)

// Excludes the characters §6.1 excludes, for the same reason: this gets read
// off a screen and typed once.
const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

// 20 characters of this alphabet is ~116 bits. The password is short-lived,
// but it is an admin credential in the meantime.
const length = 20

func main() {
	password := make([]byte, length)
	for i := range password {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			panic(err)
		}
		password[i] = alphabet[n.Int64()]
	}

	hash, err := auth.HashPassword(context.Background(), string(password))
	if err != nil {
		panic(err)
	}
	fmt.Printf("PASSWORD=%s\n", password)
	fmt.Printf("HASH=%s\n", hash)
}
