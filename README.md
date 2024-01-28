# goioc/retro: Handy retry-library
[![goioc](https://habrastorage.org/webt/ym/pu/dc/ympudccm7j7a3qex_jjroxgsiwg.png)](https://github.com/goioc)

[![Go](https://github.com/goioc/retro/workflows/Go/badge.svg)](https://github.com/goioc/retro/actions)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/goioc/retro/?tab=doc)
[![CodeFactor](https://www.codefactor.io/repository/github/goioc/retro/badge)](https://www.codefactor.io/repository/github/goioc/retro)
[![Go Report Card](https://goreportcard.com/badge/github.com/goioc/retro)](https://goreportcard.com/report/github.com/goioc/retro)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=goioc_retro&metric=alert_status)](https://sonarcloud.io/dashboard?id=goioc_retro)
[![codecov](https://codecov.io/gh/goioc/retro/graph/badge.svg?token=5TgRXVHyP1)](https://codecov.io/gh/goioc/retro)
[![DeepSource](https://static.deepsource.io/deepsource-badge-light-mini.svg)](https://deepsource.io/gh/goioc/retro/?ref=repository-badge)

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/G2G5JUKU7)

## Why another retry library?

There's a bunch of excellent retry-related Go libraries out there (and I took inspiration) from some of them, like [this one] (https://github.com/sethvargo/go-retry). Some of them are highly configurable (even more configurable than mine), but I was always lacking one feature: configurability by the error. I.e., most of the libraries allow you to configure the "retrier" in some or another way, making it behave the same way for all the "retriable" errors. What I needed for some of my usecases, if having different retry strategies for different errore. And that's why this library was made.

## Basic usage

So, let's say we have a DB-related function that may return different types of errors: it returns `sql.ErrNoRows` if the result-set of the lookup is empty, and it returns `driver.ErrBadConn` if there's some trasient connectivity error. In the first case, I want to retry with the constant rate (let's say, every `second`) and the maximum retry count of `3`. In the second case, I want to have an exponential back-off starting with `10 milliseconds`, but the retry delay should not exceed `1 second`. All together, I want the retry phase to not exceed `10 seconds`. Here's how one could implement it using the `retro` library:
```go
	caller := NewCaller(). // instantiating the "retrier" aka "caller"
		WithRetriableError(sql.ErrNoRows,
			NewBackoffStrategy(NewConstant(1), time.Second).
				WithMaxRetries(3)). // constant back-off at the rate of 1 second and 3 max retries for sql.ErrNoRows
		WithRetriableError(driver.ErrBadConn,
			NewBackoffStrategy(NewExponential(2), time.Millisecond).
				WithCappedDuration(time.Second)). // exponential back-off with factor 2, 1 millisecond time unit and max retry delay of 1 second
		WithMaxDuration(10 * time.Second) // maximum retry duration (across all retriable errors) - 10 seconds
	if err := caller.Call(context.TODO(), func() error {
		return queryDBFunction(...) // your function that runs a database query
	}); err != nil {
		panic(err)
	}
```

## More examples?

Please, take a look at the unit-tests for more examples.
