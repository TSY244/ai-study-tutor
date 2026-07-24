// Package clock provides an injectable time source so domain and
// application code never calls time.Now directly and stays testable.
package clock

import "time"

// Clock supplies the current time. Implementations must return UTC.
type Clock interface {
	Now() time.Time
}

// System is the production clock backed by time.Now.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

// Fixed is a deterministic clock for tests.
type Fixed struct {
	T time.Time
}

func (f Fixed) Now() time.Time { return f.T.UTC() }
