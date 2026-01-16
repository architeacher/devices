package circuitbreaker

import "errors"

// Sentinel errors for circuit breaker states.
var (
	// ErrCircuitOpen indicates the circuit breaker is in open state,
	// rejecting all requests to allow the downstream service to recover.
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrTooManyRequests indicates the circuit breaker is in half-open state
	// and the maximum number of probe requests has been reached.
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)
