package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	cb := New(3, 2, time.Second)

	if cb.State() != StateClosed {
		t.Errorf("expected closed, got %v", cb.State())
	}

	err := cb.Call(func() error {
		return errors.New("test error")
	})
	if err == nil {
		t.Error("expected error")
	}

	for i := 0; i < 2; i++ {
		cb.Call(func() error {
			return errors.New("test error")
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected open, got %v", cb.State())
	}

	err = cb.Call(func() error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Errorf("expected circuit open error, got %v", err)
	}

	time.Sleep(time.Second * 2)

	err = cb.Call(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected success in half-open, got %v", err)
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("expected half-open, got %v", cb.State())
	}
}

