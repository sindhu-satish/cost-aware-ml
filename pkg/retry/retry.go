package retry

import (
	"math"
	"math/rand"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func DefaultConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     2 * time.Second,
		Multiplier:   2.0,
	}
}

func Retry(config RetryConfig, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		if attempt < config.MaxAttempts-1 {
			delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt)))
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			
			jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
			time.Sleep(delay + jitter)
		}
	}
	
	return lastErr
}

