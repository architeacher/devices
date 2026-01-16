package circuitbreaker

import "time"

// Config holds the configuration for a circuit breaker.
type Config struct {
	// Name identifies the circuit breaker in logs and metrics.
	Name string

	// Enabled determines whether the circuit breaker is active.
	// When false, New returns nil and Execute passes through directly.
	Enabled bool

	// MaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open. If MaxRequests is 0,
	// the circuit breaker allows only 1 request.
	MaxRequests uint

	// Interval is the cyclic period of the closed state for the circuit breaker
	// to clear the internal counts. If Interval is 0, the circuit breaker
	// doesn't clear internal counts during the closed state.
	Interval time.Duration

	// Timeout is the period of the open state, after which the state of the
	// circuit breaker becomes half-open. If Timeout is 0, the timeout value
	// defaults to 60 seconds.
	Timeout time.Duration

	// FailureThreshold is the number of consecutive failures required to
	// trip the circuit breaker from closed to open state.
	FailureThreshold uint
}
