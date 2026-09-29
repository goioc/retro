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
	"context"
	"errors"
	"time"
)

// Caller calls functions with the configured retry strategies. Configuration
// methods return independent callers. A configured Caller can be reused and its
// Call method can be used concurrently.
type Caller struct {
	retriableErrors  []retryRule
	anyErrorStrategy *BackoffStrategy
	maxDuration      *time.Duration
}

type retryRule struct {
	err      error
	strategy BackoffStrategy
}

// NewCaller constructs a Caller. The zero value is also ready to use.
func NewCaller() Caller {
	return Caller{}
}

// WithRetriableError registers a strategy for errors matching err via errors.Is.
// Rules are matched in registration order; the first matching rule is used.
// Each rule has its own retry budget and generator for each Call.
func (c Caller) WithRetriableError(err error, strategy BackoffStrategy) Caller {
	rules := make([]retryRule, len(c.retriableErrors)+1)
	copy(rules, c.retriableErrors)
	rules[len(c.retriableErrors)] = retryRule{err: err, strategy: strategy}
	c.retriableErrors = rules
	return c
}

// WithRetryOnAnyError retries all errors using strategy. When configured, it
// takes precedence over the rules registered with WithRetriableError.
func (c Caller) WithRetryOnAnyError(strategy BackoffStrategy) Caller {
	c.anyErrorStrategy = &strategy
	return c
}

// WithMaxDuration limits the time spent calling f and waiting between attempts.
// It interrupts backoff waits, but cannot interrupt f while it is executing.
// A nonpositive duration prevents f from being called.
func (c Caller) WithMaxDuration(maxDuration time.Duration) Caller {
	c.maxDuration = &maxDuration
	return c
}

// Call invokes f until it succeeds, returns an unregistered error, exhausts its
// retry budget, or ctx is canceled. Retry state is independent for every Call.
// On cancellation, the returned error matches ctx.Err() and, if present, the
// last error from f through errors.Is. A successful f returns nil.
//
// Call does not run f in a separate goroutine. To cancel work inside f, have f
// observe its own context (including any deadline needed for that work).
func (c Caller) Call(ctx context.Context, f func() error) error {
	if c.maxDuration != nil {
		ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, *c.maxDuration)
		defer cancelFunc()
		ctx = ctxWithTimeout
	}
	// Only execution state is copied; registered configuration stays immutable.
	rules := make([]retryRule, len(c.retriableErrors))
	for i, rule := range c.retriableErrors {
		rules[i] = retryRule{err: rule.err, strategy: rule.strategy.fresh()}
	}
	var anyStrategy *BackoffStrategy
	if c.anyErrorStrategy != nil {
		strategy := c.anyErrorStrategy.fresh()
		anyStrategy = &strategy
	}

	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(lastErr, err)
		}
		lastErr = f()
		if lastErr == nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return errors.Join(lastErr, err)
		}

		strategy := anyStrategy
		if strategy == nil {
			for i := range rules {
				if errors.Is(lastErr, rules[i].err) {
					strategy = &rules[i].strategy
					break
				}
			}
		}
		if strategy == nil {
			return lastErr
		}
		delay, err := strategy.Delay()
		if errors.Is(err, ErrMaxRetries) {
			return lastErr
		}
		if err != nil {
			return errors.Join(lastErr, err)
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(lastErr, ctx.Err())
		case <-timer.C:
		}
	}
}
