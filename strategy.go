/*
 * Copyright (c) 2024 Go IoC
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 */

package retro

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

// ErrMaxRetries indicates that a strategy's retry budget has been exhausted.
var ErrMaxRetries = errors.New("reached max retries")

// BackoffStrategy configures delays and retry limits. Caller starts a fresh
// sequence for every Call. Direct calls to Delay advance one sequence; copies
// of a strategy share that direct sequence and its retry counter.
type BackoffStrategy struct {
	generatorFactory            func() Generator
	durationUnit                time.Duration
	jitterInNanoseconds         int64
	cappedDurationInNanoseconds int64
	maxRetries                  int64

	state *backoffState
}

type backoffState struct {
	mu        sync.Mutex
	called    int64
	generator Generator
}

// NewBackoffStrategy multiplies generator's sequence by durationUnit. Built-in
// generators start from the beginning for each Call, regardless of prior use.
// Custom generators can be used directly with Delay; to use one with Caller,
// use NewBackoffStrategyWithFactory so each Call gets independent state.
// Negative duration units are reported as errors by Delay.
func NewBackoffStrategy(generator Generator, durationUnit time.Duration) BackoffStrategy {
b := NewBackoffStrategyWithFactory(nil, durationUnit)
	b.state.generator = generator
	if g, ok := generator.(interface{ newGenerator() Generator }); ok {
		b.generatorFactory = g.newGenerator
	}
	return b
}

// NewBackoffStrategyWithFactory constructs a strategy for a custom generator.
// factory must be safe to call concurrently and return a new, non-nil generator
// with independent state each time. It is called lazily on the first retry.
func NewBackoffStrategyWithFactory(factory func() Generator, durationUnit time.Duration) BackoffStrategy {
	return BackoffStrategy{
		generatorFactory:            factory,
		durationUnit:                durationUnit,
		cappedDurationInNanoseconds: math.MaxInt64,
		maxRetries:                  math.MaxInt64,
		state:                       &backoffState{},
	}
}

// WithJitter adds uniform jitter in [-jitter, jitter) to each delay. Zero
// disables jitter. Negative values are reported as errors by Delay.
func (b BackoffStrategy) WithJitter(jitter time.Duration) BackoffStrategy {
	b.jitterInNanoseconds = jitter.Nanoseconds()
	return b
}

// WithCappedDuration caps the final delay, including jitter. Negative caps are
// reported as errors by Delay.
func (b BackoffStrategy) WithCappedDuration(cappedDuration time.Duration) BackoffStrategy {
	b.cappedDurationInNanoseconds = cappedDuration.Nanoseconds()
	return b
}

// WithMaxRetries permits maxRetries retries after the initial attempt, per
// Call and per registered rule. Zero disables retries. Negative limits are
// reported as errors by Delay.
func (b BackoffStrategy) WithMaxRetries(maxRetries int64) BackoffStrategy {
	b.maxRetries = maxRetries
	return b
}

func (b BackoffStrategy) fresh() BackoffStrategy {
	b.state = &backoffState{}
	return b
}

// Delay returns the next delay, or ErrMaxRetries once the configured number of
// delays has been returned. It reports invalid configuration as an error.
// Concurrent calls to Delay on the same strategy are serialized.
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
