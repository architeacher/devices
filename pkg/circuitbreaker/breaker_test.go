package circuitbreaker

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cfg      Config
		wantNil  bool
		wantName string
	}{
		{
			name: "creates circuit breaker when enabled",
			cfg: Config{
				Name:             "test-service",
				Enabled:          true,
				MaxRequests:      5,
				Interval:         60 * time.Second,
				Timeout:          30 * time.Second,
				FailureThreshold: 5,
			},
			wantNil:  false,
			wantName: "test-service",
		},
		{
			name: "returns nil when disabled",
			cfg: Config{
				Name:    "disabled-service",
				Enabled: false,
			},
			wantNil: true,
		},
		{
			name: "creates with zero max requests defaults to 1",
			cfg: Config{
				Name:             "zero-max",
				Enabled:          true,
				MaxRequests:      0,
				Interval:         60 * time.Second,
				Timeout:          30 * time.Second,
				FailureThreshold: 3,
			},
			wantNil:  false,
			wantName: "zero-max",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cb := New[string](tc.cfg)

			if tc.wantNil {
				require.Nil(t, cb)

				return
			}

			require.NotNil(t, cb)
			require.Equal(t, tc.wantName, cb.Name())
		})
	}
}

func TestExecute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		cb        *CircuitBreaker[string]
		fn        func() (string, error)
		wantVal   string
		wantErr   error
		errSubstr string
	}{
		{
			name: "executes successfully with circuit breaker",
			cb: New[string](Config{
				Name:             "success-test",
				Enabled:          true,
				MaxRequests:      5,
				Interval:         60 * time.Second,
				Timeout:          30 * time.Second,
				FailureThreshold: 5,
			}),
			fn: func() (string, error) {
				return "success", nil
			},
			wantVal: "success",
			wantErr: nil,
		},
		{
			name: "passes through when circuit breaker is nil",
			cb:   nil,
			fn: func() (string, error) {
				return "direct", nil
			},
			wantVal: "direct",
			wantErr: nil,
		},
		{
			name: "returns error from function",
			cb: New[string](Config{
				Name:             "error-test",
				Enabled:          true,
				MaxRequests:      5,
				Interval:         60 * time.Second,
				Timeout:          30 * time.Second,
				FailureThreshold: 5,
			}),
			fn: func() (string, error) {
				return "", errors.New("operation failed")
			},
			wantVal:   "",
			errSubstr: "operation failed",
		},
		{
			name: "nil circuit breaker returns error from function",
			cb:   nil,
			fn: func() (string, error) {
				return "", errors.New("direct error")
			},
			wantVal:   "",
			errSubstr: "direct error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := Execute(tc.cb, tc.fn)

			if tc.errSubstr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errSubstr)
			} else if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantVal, result)
		})
	}
}

func TestCircuitBreaker_OpenState(t *testing.T) {
	t.Parallel()

	cb := New[string](Config{
		Name:             "open-state-test",
		Enabled:          true,
		MaxRequests:      1,
		Interval:         1 * time.Second,
		Timeout:          1 * time.Second,
		FailureThreshold: 1,
	})
	require.NotNil(t, cb)

	// First call fails, tripping the breaker.
	_, err := Execute(cb, func() (string, error) {
		return "", errors.New("failure")
	})
	require.Error(t, err)

	// Second call should be rejected with ErrCircuitOpen.
	_, err = Execute(cb, func() (string, error) {
		return "should not execute", nil
	})

	require.Error(t, err)
	require.ErrorIs(t, err, ErrCircuitOpen)
}

func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	t.Parallel()

	cb := New[string](Config{
		Name:             "half-open-test",
		Enabled:          true,
		MaxRequests:      1,
		Interval:         100 * time.Millisecond,
		Timeout:          100 * time.Millisecond,
		FailureThreshold: 1,
	})
	require.NotNil(t, cb)

	// Trip the breaker.
	_, _ = Execute(cb, func() (string, error) {
		return "", errors.New("failure")
	})

	// Wait for timeout to transition to half-open.
	time.Sleep(150 * time.Millisecond)

	// First request in half-open should go through.
	result, err := Execute(cb, func() (string, error) {
		return "recovered", nil
	})
	require.NoError(t, err)
	require.Equal(t, "recovered", result)
}

func TestCircuitBreaker_TooManyRequests(t *testing.T) {
	t.Parallel()

	cb := New[string](Config{
		Name:             "too-many-test",
		Enabled:          true,
		MaxRequests:      1,
		Interval:         100 * time.Millisecond,
		Timeout:          100 * time.Millisecond,
		FailureThreshold: 1,
	})
	require.NotNil(t, cb)

	// Trip the breaker.
	_, _ = Execute(cb, func() (string, error) {
		return "", errors.New("failure")
	})

	// Wait for timeout to transition to half-open.
	time.Sleep(150 * time.Millisecond)

	// Start first request (allowed in half-open).
	started := make(chan struct{})
	done := make(chan struct{})

	go func() {
		close(started)
		_, _ = Execute(cb, func() (string, error) {
			time.Sleep(50 * time.Millisecond)

			return "slow", nil
		})
		close(done)
	}()

	<-started
	time.Sleep(10 * time.Millisecond)

	// Second concurrent request should be rejected.
	_, err := Execute(cb, func() (string, error) {
		return "should not run", nil
	})

	require.Error(t, err)
	require.ErrorIs(t, err, ErrTooManyRequests)

	<-done
}

func TestCircuitBreaker_GenericTypes(t *testing.T) {
	t.Parallel()

	type Response struct {
		ID   int
		Name string
	}

	cb := New[*Response](Config{
		Name:             "generic-test",
		Enabled:          true,
		MaxRequests:      5,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		FailureThreshold: 3,
	})
	require.NotNil(t, cb)

	result, err := Execute(cb, func() (*Response, error) {
		return &Response{ID: 1, Name: "test"}, nil
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ID)
	require.Equal(t, "test", result.Name)
}

func TestCircuitBreaker_NilResult(t *testing.T) {
	t.Parallel()

	cb := New[*string](Config{
		Name:             "nil-result-test",
		Enabled:          true,
		MaxRequests:      5,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		FailureThreshold: 3,
	})
	require.NotNil(t, cb)

	result, err := Execute(cb, func() (*string, error) {
		return nil, nil
	})

	require.NoError(t, err)
	require.Nil(t, result)
}
