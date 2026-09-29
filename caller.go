package retro

import (
	"context"
	"errors"
	"time"
)

// Caller invokes functions according to its retry rules. Its configuration
// methods return new callers without changing the receiver. A configured Caller
// can be reused and its Call method can be used concurrently.
// The zero value calls a function once without retrying failures.
type Caller struct {
	retriableErrors  []retryRule
	anyErrorStrategy *BackoffStrategy
	maxDuration      *time.Duration
}

// retryRule pairs an error target with a strategy. Slice order determines which
// rule wins when more than one target matches an error.
type retryRule struct {
	err      error
	strategy BackoffStrategy
}

// NewCaller returns a caller with no retry rules or timeout, equivalent to the
// zero value of [Caller].
func NewCaller() Caller {
	return Caller{}
}

// WithRetriableError returns a caller with a strategy for errors that satisfy
// errors.Is(returnedError, err), including wrapped and joined errors.
// The first matching rule in registration order wins. Each rule has its own
// retry budget and generator for each Call. [Caller.WithRetryOnAnyError], when
// configured, takes precedence over these rules.
func (c Caller) WithRetriableError(err error, strategy BackoffStrategy) Caller {
	rules := make([]retryRule, len(c.retriableErrors)+1)
	copy(rules, c.retriableErrors)
	rules[len(c.retriableErrors)] = retryRule{err: err, strategy: strategy}
	c.retriableErrors = rules
	return c
}

// WithRetryOnAnyError returns a caller that retries every error using strategy.
// It takes precedence over rules registered with [Caller.WithRetriableError].
func (c Caller) WithRetryOnAnyError(strategy BackoffStrategy) Caller {
	c.anyErrorStrategy = &strategy
	return c
}

// WithMaxDuration returns a caller with a timeout measured from the start of
// each Call. An earlier deadline on Call's context takes precedence.
// Expiration interrupts backoff waits, but a running callback may continue past
// the timeout. A nonpositive duration prevents the initial attempt.
func (c Caller) WithMaxDuration(maxDuration time.Duration) Caller {
	c.maxDuration = &maxDuration
	return c
}

// Call invokes f synchronously and retries failures according to the caller's
// rules. Every Call starts fresh retry counters and generator sequences.
// It returns nil when f succeeds, or the last error from f when no rule matches
// or the selected strategy's retry budget is exhausted.
//
// Cancellation, a caller timeout, or invalid strategy configuration stops
// retries. The returned error matches both the stopping error and the last
// error from f, when one exists, through [errors.Is]. An already-canceled
// context prevents the initial attempt.
//
// Cancellation interrupts waits, but cannot interrupt an executing f. Pass a
// context to the work inside f if it should also stop on cancellation. A timeout
// set by [Caller.WithMaxDuration] is internal to Call and is not passed to f.
// Neither ctx nor f may be nil.
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
