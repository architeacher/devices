package backoff

import (
	"math/rand"
	"time"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/config"
)

type (
	// Strategy defines the methodology for backing off after a grpc connection
	// failure.
	Strategy interface {
		// Backoff returns the amount of time to wait before the next retry given
		// the number of consecutive failures.
		Backoff(retries int) time.Duration
	}

	// Exponential implements exponential backoff algorithm.
	Exponential struct {
		// config contains all options to configure the backoff algorithm.
		config config.Backoff
	}
)

func NewExponentialStrategy(cfg config.Backoff) Exponential {
	return Exponential{
		config: cfg,
	}
}

// Backoff calculates the backoff duration using exponential backoff with jitter.
func (bc Exponential) Backoff(retries int) time.Duration {
	if retries == 0 {
		return bc.config.BaseDelay
	}

	backoff, maxBackoff := float64(bc.config.BaseDelay), float64(bc.config.MaxDelay)
	for backoff < maxBackoff && retries > 0 {
		backoff *= bc.config.Multiplier
		retries--
	}

	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	backoff *= 1 + bc.config.Jitter*(rand.Float64()*2-1)
	if backoff < 0 {
		backoff = 0
	}

	return time.Duration(backoff)
}
