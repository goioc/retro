# goioc/retro: Handy retry-library
[![goioc](https://habrastorage.org/webt/ym/pu/dc/ympudccm7j7a3qex_jjroxgsiwg.png)](https://github.com/goioc)

[![Go](https://github.com/goioc/retro/workflows/Go/badge.svg)](https://github.com/goioc/retro/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/goioc/retro/?tab=doc)
[![codecov](https://codecov.io/gh/goioc/retro/graph/badge.svg?token=5TgRXVHyP1)](https://codecov.io/gh/goioc/retro)

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/G2G5JUKU7)

## Why another retry library?

There's a bunch of excellent retry-related Go libraries out there (and I took inspiration from some of them), like [this one](https://github.com/sethvargo/go-retry). Some of them are highly configurable (even more configurable than mine), but I was always lacking one feature: configurability by the error. I.e., most of the libraries allow you to configure the "retrier" in some or another way, making it behave the same way for all the "retriable" errors. What I needed for some of my usecases, is having different retry strategies for different errors. And that's why this library was made.

## Basic usage

So, let's say we have a DB-related function that may return different types of errors: it returns `sql.ErrNoRows` if the result-set of the lookup is empty, and it returns `driver.ErrBadConn` if there's some transient connectivity error. In the first case, I want to retry with the constant rate (let's say, every `second`) and the maximum retry count of `3`. In the second case, I want to have an exponential back-off starting with `10 milliseconds`, but the retry delay should not exceed `1 second`. I want to stop retrying after `5 seconds`. Here's how one could implement it using the `retro` library:

```go
	caller := NewCaller(). // instantiating the "retrier" aka "caller"
		WithRetriableError(sql.ErrNoRows,
			NewBackoffStrategy(NewConstant(1), time.Second).
				WithMaxRetries(3)). // constant back-off at the rate of 1 second and 3 max retries for sql.ErrNoRows
		WithRetriableError(driver.ErrBadConn,
			NewBackoffStrategy(NewExponential(2), 10*time.Millisecond).
				WithCappedDuration(time.Second)). // exponential back-off starting at 10 milliseconds with a 1-second cap
		WithMaxDuration(5 * time.Second) // maximum retry duration (across all retriable errors) - 5 seconds
	if err := caller.Call(context.TODO(), func() error {
		return queryDBFunction(...) // your function that runs a database query
	}); err != nil {
		panic(err)
	}
```

You can also specify a retry strategy for any error using `WithRetryOnAnyError(...)` on the caller. When configured, this takes precedence over error-specific rules.

## Retry behavior

- `WithMaxRetries(N)` allows the initial attempt plus **N retries**. This corrects the previous behavior, which counted the initial attempt toward the limit. Zero disables retries.
- Error-specific rules use `errors.Is(returnedError, registeredError)`, including wrapped and joined errors. The first matching rule in registration order wins.
- Callers can be reused and called concurrently. Every `Call` starts fresh counters and generator sequences, with a separate budget for each registered rule. Configuration methods return independent callers.
- Cancellation and `WithMaxDuration` interrupt backoff waits. Cancellation errors match both the context error and the last operation error, when one exists, using `errors.Is`. An already-canceled context prevents the initial attempt.
- The callback runs synchronously. Neither cancellation nor `WithMaxDuration` can forcibly stop it. Pass an appropriate context to the work inside the callback; use a parent context with a timeout when the same deadline should apply to both the work and retries.
- Growing sequences and duration arithmetic saturate at the `int64` bounds instead of wrapping. Delays are clamped to zero and the configured cap after jitter.

## Custom generators and validation

Built-in generators work with `NewBackoffStrategy` as before. Custom generators used with `Caller` must use `NewBackoffStrategyWithFactory` so every call can create independent state:

```go
strategy := NewBackoffStrategyWithFactory(func() Generator {
	return &myGenerator{} // a fresh generator for every call
}, time.Millisecond).WithMaxRetries(3)

caller := NewCaller().WithRetryOnAnyError(strategy)
```

The factory must be safe to invoke concurrently and return a new, non-nil generator each time. A custom generator passed directly to `NewBackoffStrategy` still supports direct `Delay()` calls; using it with `Caller` returns a configuration error when a retry is needed. Direct `Delay()` calls on copies of the same strategy share one sequence and are serialized.

`NewRandom(max)` requires a positive maximum and panics at construction for invalid bounds, matching `rand.Int63n`'s contract. Negative duration units, jitter, caps, and retry limits are reported as errors by `Delay()` (and propagated by `Call` when retrying). Jitter supports the full nonnegative `time.Duration` range. `NewExponential` starts at 1; `NewLinear` starts at 0.

## More examples?

Please, take a look at the unit-tests for more examples.
