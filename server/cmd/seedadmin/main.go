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
