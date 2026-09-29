package retro

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

// ErrMaxRetries is returned by [BackoffStrategy.Delay] when its retry budget is
// exhausted. [Caller.Call] returns the last callback error instead.
var ErrMaxRetries = errors.New("reached max retries")

// BackoffStrategy configures delays and retry limits. Use [NewBackoffStrategy]
// or [NewBackoffStrategyWithFactory] to initialize it.
//
// Caller starts a fresh sequence for every Call. Direct Delay calls advance one
// sequence; copies of a strategy, including values returned by its configuration
// methods, share that direct sequence and its retry counter.
type BackoffStrategy struct {
	generatorFactory            func() Generator
	durationUnit                time.Duration
	jitterInNanoseconds         int64
	cappedDurationInNanoseconds int64
	maxRetries                  int64

	state *backoffState
}

// backoffState serializes access to one generator sequence and retry counter.
type backoffState struct {
	mu        sync.Mutex
	called    int64
	generator Generator
}

// NewBackoffStrategy returns a strategy whose base delays are generator values
// multiplied by durationUnit. It defaults to no jitter, [math.MaxInt64] retries,
// and a maximum delay of math.MaxInt64 nanoseconds.
//
// Direct Delay calls advance the supplied generator from its current position.
// Caller restarts built-in generators for each Call, regardless of prior use.
// For custom generators used with Caller, use [NewBackoffStrategyWithFactory]
// to provide independent state. Delay reports negative duration units as errors.
func NewBackoffStrategy(generator Generator, durationUnit time.Duration) BackoffStrategy {
	b := NewBackoffStrategyWithFactory(nil, durationUnit)
	b.state.generator = generator
	if g, ok := generator.(interface{ newGenerator() Generator }); ok {
		b.generatorFactory = g.newGenerator
	}
	return b
}

// NewBackoffStrategyWithFactory returns a strategy that creates generators on
// demand, with the same defaults as [NewBackoffStrategy]. The factory is called
// lazily when a sequence first needs a generator, including each rule in a Call.
// It must be safe to invoke concurrently and return a new, non-nil generator
// with independent state each time. A nil factory, nil result, or negative
// duration unit is reported as an error by Delay.
func NewBackoffStrategyWithFactory(factory func() Generator, durationUnit time.Duration) BackoffStrategy {
	return BackoffStrategy{
		generatorFactory:            factory,
		durationUnit:                durationUnit,
		cappedDurationInNanoseconds: math.MaxInt64,
		maxRetries:                  math.MaxInt64,
		state:                       &backoffState{},
	}
}

// WithJitter returns a strategy that adds uniform jitter in [-jitter, jitter)
// to each base delay before clamping it to zero and the configured cap.
// Zero disables jitter. Delay reports negative values as errors.
func (b BackoffStrategy) WithJitter(jitter time.Duration) BackoffStrategy {
	b.jitterInNanoseconds = jitter.Nanoseconds()
	return b
}

// WithCappedDuration returns a strategy that limits the final delay, including
// jitter, to cappedDuration. A zero cap makes all delays zero. Delay reports
// negative caps as errors.
func (b BackoffStrategy) WithCappedDuration(cappedDuration time.Duration) BackoffStrategy {
	b.cappedDurationInNanoseconds = cappedDuration.Nanoseconds()
	return b
}

// WithMaxRetries returns a strategy permitting maxRetries retries after the
// initial attempt, per Call and per registered rule. Direct use permits that
// many successful Delay calls. Zero disables retries; Delay reports negative
// limits as errors.
func (b BackoffStrategy) WithMaxRetries(maxRetries int64) BackoffStrategy {
	b.maxRetries = maxRetries
	return b
}

// fresh retains configuration but discards execution state. A factory supplies
// a new generator when this Call first needs a delay.
func (b BackoffStrategy) fresh() BackoffStrategy {
	b.state = &backoffState{}
	return b
}

// Delay returns the next delay, clamped to zero and the configured cap after
// adding jitter. Duration arithmetic saturates at the int64 bounds on overflow.
// It returns [ErrMaxRetries] after the configured number of delays, or an error
// for invalid configuration. On error, the returned duration is zero.
// Concurrent calls on the same strategy or its copies are serialized.
func (b BackoffStrategy) Delay() (time.Duration, error) {
	if b.state == nil {
		return 0, errors.New("retro: initialize the strategy with a constructor")
	}
	if b.durationUnit < 0 || b.jitterInNanoseconds < 0 || b.cappedDurationInNanoseconds < 0 || b.maxRetries < 0 {
		return 0, errors.New("retro: duration unit, jitter, cap, and retry limit must be nonnegative")
	}
	b.state.mu.Lock()
	defer b.state.mu.Unlock()
	if b.state.called >= b.maxRetries {
		return 0, ErrMaxRetries
	}
	if b.state.generator == nil {
		if b.generatorFactory == nil {
			return 0, errors.New("retro: a generator factory is required; use NewBackoffStrategyWithFactory for custom generators in Caller")
		}
		b.state.generator = b.generatorFactory()
		if b.state.generator == nil {
			return 0, errors.New("retro: generator factory returned nil")
		}
	}
	step := b.state.generator.Next()
	b.state.called++
	durationInNanoseconds := saturatingMultiply(step, b.durationUnit.Nanoseconds())
	jitterInNanoseconds := int64(0)
	if b.jitterInNanoseconds != 0 {
		// Sample the sign separately so even MaxInt64 jitter cannot overflow.
		jitterInNanoseconds = rand.Int63n(b.jitterInNanoseconds)
		if rand.Intn(2) == 0 {
			jitterInNanoseconds = -jitterInNanoseconds - 1
		}
	}
	durationInNanoseconds = saturatingAdd(durationInNanoseconds, jitterInNanoseconds)
	if durationInNanoseconds < 0 {
		durationInNanoseconds = 0
	} else if durationInNanoseconds > b.cappedDurationInNanoseconds {
		durationInNanoseconds = b.cappedDurationInNanoseconds
	}
	return time.Duration(durationInNanoseconds), nil
}
