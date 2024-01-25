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

// Caller is the main type of the library, responsible for calling a specified function, following the specified retry
// strategy.
type Caller struct {
	retriableErrors  map[error]BackoffStrategy
	anyErrorStrategy *BackoffStrategy
	maxDuration      *time.Duration
}

// NewCaller is just a constructor for Caller.
func NewCaller() Caller {
	return Caller{
		retriableErrors: map[error]BackoffStrategy{},
	}
}

// WithRetriableError allows one to register an error along with the backoff strategy for it.
func (c Caller) WithRetriableError(err error, strategy BackoffStrategy) Caller {
	for e := range c.retriableErrors {
		if errors.Is(e, err) {
			return c
		}
	}
	c.retriableErrors[err] = strategy
	return c
}

// WithRetryOnAnyError acts like WithRetriableError, but the specified strategy will be used upon getting _any_ error (
// i.e. it makes all errors retriable).
func (c Caller) WithRetryOnAnyError(strategy BackoffStrategy) Caller {
	c.anyErrorStrategy = &strategy
	return c
}

// WithMaxDuration allow one to specify maximum duration (total for all retries).
func (c Caller) WithMaxDuration(maxDuration time.Duration) Caller {
	c.maxDuration = &maxDuration
	return c
}

// Call is the main method that accepts a context.Context (which can be used to terminate retrying) and the function,
// which is essentially a wrapper around some actual function.
func (c Caller) Call(ctx context.Context, f func() error) (err error) {
	if c.maxDuration != nil {
		ctxWithTimeout, cancelFunc := context.WithTimeout(ctx, *c.maxDuration)
		defer cancelFunc()
		ctx = ctxWithTimeout
	}
mainLoop:
	for {
		select {
		case <-ctx.Done():
			return err
		default:
			if err = f(); err == nil {
				return nil
			}
			if c.anyErrorStrategy != nil {
				delay, e := c.anyErrorStrategy.Delay()
				if e != nil {
					return err
				}
				time.Sleep(delay)
				continue
			}
			for e, s := range c.retriableErrors {
				if errors.Is(e, err) {
					delay, e := s.Delay()
					if e != nil {
						return err
					}
					time.Sleep(delay)
					continue mainLoop
				}
			}
			return err
		}
	}
}
