// Package id provides injectable identifier generation (UUID v4) and
// request-ID validation. Domain/application code depends on the Generator
// interface, never on a concrete source, so tests stay deterministic.
package id

import (
	"strings"

	"github.com/google/uuid"
)

// Generator creates new unique IDs (UUID v4 strings).
type Generator interface {
	New() string
}

// Random is the production generator backed by crypto/rand UUIDs.
type Random struct{}

func (Random) New() string { return uuid.NewString() }

// Fixed returns deterministic IDs for tests (cycles through Values).
type Fixed struct {
	Values []string
	next   int
}

func (f *Fixed) New() string {
	if len(f.Values) == 0 {
		return "00000000-0000-4000-8000-000000000000"
	}
	v := f.Values[f.next%len(f.Values)]
	f.next++
	return v
}

// MaxRequestIDLength bounds client-supplied X-Request-ID values.
const MaxRequestIDLength = 128

// IsValidRequestID reports whether a client-supplied X-Request-ID is
// acceptable: printable ASCII (0x21–0x7E), bounded length. Invalid values
// are replaced by the server instead of being reflected.
func IsValidRequestID(value string) bool {
	if value == "" || len(value) > MaxRequestIDLength {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7E {
			return false
		}
	}
	return true
}

// SanitizeRequestID returns the client value when valid, otherwise a fresh
// server-generated ID. The result is always safe to echo and log.
func SanitizeRequestID(value string, gen Generator) string {
	if IsValidRequestID(value) {
		return strings.Clone(value)
	}
	return gen.New()
}
