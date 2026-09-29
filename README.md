# retro

[![Go](https://github.com/goioc/retro/workflows/Go/badge.svg)](https://github.com/goioc/retro/actions)
[![API reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/goioc/retro/)
[![codecov](https://codecov.io/gh/goioc/retro/graph/badge.svg?token=5TgRXVHyP1)](https://codecov.io/gh/goioc/retro)

A Go retry library with **different backoff strategies for different errors**. Use constant, random, linear, exponential, or Fibonacci delays, with retry limits, jitter, and context cancellation. The library has no dependencies outside the Go standard library at runtime.

## Install

Requires Go 1.21 or later.

```sh
go get github.com/goioc/retro
```

## Quickstart

Retry a temporary failure every 100 ms, allowing up to three retries after the initial attempt. Use callbacks that can safely run more than once.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goioc/retro"
)

func main() {
	caller := retro.NewCaller().WithRetryOnAnyError(
		retro.NewBackoffStrategy(retro.NewConstant(1), 100*time.Millisecond).
			WithMaxRetries(3),
	)

	attempts := 0
	err := caller.Call(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	fmt.Printf("attempts=%d, err=%v\n", attempts, err)
	// attempts=3, err=<nil>
}
```

## Choose a backoff

`NewBackoffStrategy(generator, durationUnit)` multiplies each generated value by the duration unit.

| Generator | Values | Delays with a 100 ms unit |
| --- | --- | --- |
| `NewConstant(2)` | 2, 2, 2, … | 200 ms, 200 ms, 200 ms, … |
| `NewRandom(5)` | Random integers from 0 through 4 | 0, 100, 200, 300, or 400 ms |
| `NewLinear(1)` | 0, 1, 2, 3, … | 0 ms, 100 ms, 200 ms, 300 ms, … |
| `NewExponential(2)` | 1, 2, 4, 8, … | 100 ms, 200 ms, 400 ms, 800 ms, … |
| `NewFibonacci()` | 1, 1, 2, 3, 5, … | 100 ms, 100 ms, 200 ms, 300 ms, 500 ms, … |

Add options by chaining methods:

```go
strategy := retro.NewBackoffStrategy(retro.NewExponential(2), 100*time.Millisecond).
	WithJitter(25 * time.Millisecond).
	WithCappedDuration(time.Second).
	WithMaxRetries(5)
```

Jitter adds a random duration in `[-25 ms, +25 ms)` before the final delay is clamped between zero and the cap. Arithmetic saturates at the `int64` bounds instead of wrapping on overflow. Defaults are zero jitter, `math.MaxInt64` retries, and a cap of `time.Duration(math.MaxInt64)`.

## Retry different errors differently

Register the errors you expect and give each one its own strategy:

```go
notReady := errors.New("not ready")
busy := errors.New("busy")

caller := retro.NewCaller().
	WithRetriableError(notReady,
		retro.NewBackoffStrategy(retro.NewConstant(1), 100*time.Millisecond).
			WithMaxRetries(3)).
	WithRetriableError(busy,
		retro.NewBackoffStrategy(retro.NewExponential(2), 10*time.Millisecond).
			WithCappedDuration(time.Second).
			WithMaxRetries(5))
```

Use `caller.Call` as in the quickstart. Matching uses `errors.Is`, so `fmt.Errorf("lookup: %w", notReady)` selects the first strategy. Each rule has its own retry budget. Unmatched errors return immediately.

If several rules match, the first registered rule wins. `WithRetryOnAnyError` overrides all error-specific rules when configured.

## Cancellation and timeouts

Pass a context to both `Call` and the work inside the callback to apply the same deadline to both:

```go
func retryWithTimeout(work func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	caller := retro.NewCaller().WithRetryOnAnyError(
		retro.NewBackoffStrategy(retro.NewConstant(1), 100*time.Millisecond).
			WithMaxRetries(3),
	)
	return caller.Call(ctx, func() error {
		return work(ctx)
	})
}
```

Cancellation interrupts backoff waits immediately. The callback runs synchronously and must observe its context to stop its own work.

You can also set `caller = caller.WithMaxDuration(2 * time.Second)`. That timeout starts with each `Call` and interrupts waits, but its internal context is not passed to the callback. A callback can therefore run past this timeout. A nonpositive maximum duration, or an already-canceled context, prevents the initial attempt.

## Custom generators

Implement `Generator.Next() int64` and supply a factory that creates fresh state for each call. This example uses multipliers 1, 2, 5, then repeats 5:

```go
type schedule struct{ index int }

func (g *schedule) Next() int64 {
	steps := [...]int64{1, 2, 5}
	value := steps[g.index]
	if g.index < len(steps)-1 {
		g.index++
	}
	return value
}

func customStrategy() retro.BackoffStrategy {
	return retro.NewBackoffStrategyWithFactory(
		func() retro.Generator { return &schedule{} },
		100*time.Millisecond,
	).WithMaxRetries(3)
}
```

Pass `customStrategy()` to `WithRetryOnAnyError` or `WithRetriableError`. The factory must be safe to invoke concurrently and return a new, non-nil generator each time.

## API details

- **Retry counts:** `WithMaxRetries(N)` allows the initial attempt plus N retries per rule and per call. Zero disables retries. With no retry rules, a caller makes just one attempt.
- **Return values:** `Call` returns `nil` on success and the last operation error when retries are exhausted or no rule matches. Cancellation, timeout, and configuration errors are joined with the last operation error, if any; use `errors.Is` to inspect them. `ErrMaxRetries` is returned by direct `Delay()` calls, not by `Call` for an exhausted budget.
- **Reuse:** configured callers can be reused and called concurrently. Their configuration methods leave the original caller unchanged. Each call starts fresh counters and generator sequences.
- **Direct delays:** `strategy.Delay()` advances one sequence without invoking or sleeping for an operation. Copies of a strategy share this sequence and counter; concurrent `Delay` calls are serialized. `NewBackoffStrategy` uses the supplied generator's current state for direct delays, while `Caller` restarts built-in generators. Custom generators passed directly to that constructor support `Delay`, but require the factory constructor for use with `Caller`.
- **Validation:** `NewRandom(max)` panics at construction if `max <= 0`. Negative duration units, jitter, caps, or retry limits are reported by `Delay` and by `Call` when a retry is needed.

Upgrading from v1.0.x? Since v1.1.1, retry limits exclude the initial attempt, and custom generators used with `Caller` require a factory.

See the [API reference](https://pkg.go.dev/github.com/goioc/retro/) and [runnable examples](example_test.go) for more details. Licensed under [MIT](LICENSE).

A [Go IoC](https://github.com/goioc) project. [Support development](https://ko-fi.com/G2G5JUKU7).
