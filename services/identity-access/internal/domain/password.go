// Package domain holds identity rules that need no I/O.
package domain

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const iterations = 120_000

// HashPassword returns "pbkdf2$iter$salt$hash" (PBKDF2-HMAC-SHA256, standard library only).
func HashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	k, err := pbkdf2.Key(sha256.New, pw, salt, iterations, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pbkdf2$%d$%s$%s", iterations, hex.EncodeToString(salt), hex.EncodeToString(k)), nil
}

func CheckPassword(pw, stored string) bool {
	p := strings.Split(stored, "$")
	if len(p) != 4 || p[0] != "pbkdf2" {
		return false
	}
	it, err := strconv.Atoi(p[1])
	if err != nil {
		return false
	}
	salt, err := hex.DecodeString(p[2])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(p[3])
	if err != nil {
		return false
	}
	k, err := pbkdf2.Key(sha256.New, pw, salt, it, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(k, want) == 1
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func ValidRole(r string) bool { return r == "Controller" || r == "Responder" || r == "Citizen" }
