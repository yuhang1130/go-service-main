package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrOpen = errors.New("circuit breaker is open")

type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

type Config struct {
	FailureThreshold    uint32
	OpenTimeout         time.Duration
	HalfOpenMaxRequests uint32
}

type Option func(*Breaker)

func WithClock(now func() time.Time) Option {
	return func(b *Breaker) {
		if now != nil {
			b.now = now
		}
	}
}

type Breaker struct {
	config Config
	now    func() time.Time

	mu                sync.Mutex
	state             State
	generation        uint64
	failures          uint32
	openedAt          time.Time
	halfOpenInFlight  uint32
	halfOpenSucceeded uint32
}

type permit struct {
	state      State
	generation uint64
}

func New(config Config, options ...Option) (*Breaker, error) {
	if config.FailureThreshold == 0 {
		return nil, errors.New("circuit breaker failure threshold must be positive")
	}
	if config.OpenTimeout <= 0 {
		return nil, errors.New("circuit breaker open timeout must be positive")
	}
	if config.HalfOpenMaxRequests == 0 {
		return nil, errors.New("circuit breaker half-open max requests must be positive")
	}
	breaker := &Breaker{config: config, now: time.Now, state: StateClosed}
	for _, option := range options {
		option(breaker)
	}
	return breaker, nil
}

func (b *Breaker) Execute(ctx context.Context, operation func(context.Context) error) error {
	if operation == nil {
		return errors.New("circuit breaker operation is nil")
	}
	permit, err := b.acquire()
	if err != nil {
		return err
	}
	err = operation(ctx)
	b.complete(permit, err)
	return err
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionFromOpenIfElapsed()
	return b.state
}

func (b *Breaker) Ready(context.Context) error {
	state := b.State()
	if state != StateClosed {
		return ErrOpen
	}
	return nil
}

func (b *Breaker) acquire() (permit, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionFromOpenIfElapsed()
	switch b.state {
	case StateClosed:
		return permit{state: StateClosed, generation: b.generation}, nil
	case StateHalfOpen:
		if b.halfOpenInFlight >= b.config.HalfOpenMaxRequests {
			return permit{}, ErrOpen
		}
		b.halfOpenInFlight++
		return permit{state: StateHalfOpen, generation: b.generation}, nil
	default:
		return permit{}, ErrOpen
	}
}

func (b *Breaker) complete(permit permit, operationErr error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if permit.generation != b.generation || permit.state != b.state {
		return
	}
	switch permit.state {
	case StateClosed:
		if operationErr == nil {
			b.failures = 0
			return
		}
		b.failures++
		if b.failures >= b.config.FailureThreshold {
			b.open()
		}
	case StateHalfOpen:
		if b.halfOpenInFlight > 0 {
			b.halfOpenInFlight--
		}
		if operationErr != nil {
			b.open()
			return
		}
		b.halfOpenSucceeded++
		if b.halfOpenSucceeded >= b.config.HalfOpenMaxRequests {
			b.close()
		}
	}
}

func (b *Breaker) transitionFromOpenIfElapsed() {
	if b.state != StateOpen || b.now().Before(b.openedAt.Add(b.config.OpenTimeout)) {
		return
	}
	b.state = StateHalfOpen
	b.generation++
	b.halfOpenInFlight = 0
	b.halfOpenSucceeded = 0
}

func (b *Breaker) open() {
	b.state = StateOpen
	b.generation++
	b.openedAt = b.now()
	b.failures = 0
	b.halfOpenInFlight = 0
	b.halfOpenSucceeded = 0
}

func (b *Breaker) close() {
	b.state = StateClosed
	b.generation++
	b.failures = 0
	b.halfOpenInFlight = 0
	b.halfOpenSucceeded = 0
}
