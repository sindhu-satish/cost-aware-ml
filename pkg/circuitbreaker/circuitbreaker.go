package circuitbreaker

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu                sync.RWMutex
	state             State
	failureCount      int
	successCount      int
	lastFailureTime   time.Time
	failureThreshold  int
	successThreshold  int
	timeout           time.Duration
	halfOpenMaxCalls  int
	halfOpenCalls     int
}

func New(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		halfOpenMaxCalls: 3,
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()

	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			cb.halfOpenCalls = 0
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
	}

	if cb.state == StateHalfOpen {
		if cb.halfOpenCalls >= cb.halfOpenMaxCalls {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
		cb.halfOpenCalls++
	}

	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.state == StateHalfOpen || cb.failureCount >= cb.failureThreshold {
			cb.state = StateOpen
			cb.failureCount = 0
		}
		return err
	}

	cb.successCount++
	if cb.state == StateHalfOpen {
		if cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.successCount = 0
			cb.failureCount = 0
		}
	} else {
		cb.failureCount = 0
	}

	return nil
}

var ErrCircuitOpen = &CircuitError{Message: "circuit breaker is open"}

type CircuitError struct {
	Message string
}

func (e *CircuitError) Error() string {
	return e.Message
}

