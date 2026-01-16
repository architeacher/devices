package circuitbreaker

import (
	"errors"

	"github.com/sony/gobreaker/v2"
)

// CircuitBreaker wraps gobreaker to provide resilience for service calls.
// It uses generics to provide type-safe execution without interface boxing.
type CircuitBreaker[T any] struct {
	cb *gobreaker.CircuitBreaker[T]
}

// New creates a new circuit breaker with the given configuration.
// Returns nil if the circuit breaker is disabled in the configuration.
func New[T any](cfg Config) *CircuitBreaker[T] {
	if !cfg.Enabled {
		return nil
	}

	cb := gobreaker.NewCircuitBreaker[T](gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: uint32(cfg.MaxRequests),
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= uint32(cfg.FailureThreshold)
		},
	})

	return &CircuitBreaker[T]{cb: cb}
}

// Name returns the name of the circuit breaker.
func (c *CircuitBreaker[T]) Name() string {
	return c.cb.Name()
}

// Execute runs the given function through the circuit breaker.
// If the circuit breaker is nil, the function is executed directly.
// Returns ErrCircuitOpen when the circuit breaker is in open state.
// Returns ErrTooManyRequests when the circuit breaker is in half-open state
// and the maximum number of requests has been reached.
func Execute[T any](cb *CircuitBreaker[T], fn func() (T, error)) (T, error) {
	if cb == nil {
		return fn()
	}

	result, err := cb.cb.Execute(fn)
	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			var zero T

			return zero, ErrCircuitOpen
		}

		if errors.Is(err, gobreaker.ErrTooManyRequests) {
			var zero T

			return zero, ErrTooManyRequests
		}

		return result, err
	}

	return result, nil
}
