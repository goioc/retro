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

type Caller struct {
	retriableErrors  map[error]BackoffStrategy
	anyErrorStrategy *BackoffStrategy
	maxDuration      *time.Duration
}

func NewCaller() Caller {
	return Caller{
		retriableErrors: map[error]BackoffStrategy{},
	}
}

func (c Caller) WithRetriableError(err error, strategy BackoffStrategy) Caller {
	for e := range c.retriableErrors {
		if errors.Is(e, err) {
			return c
		}
	}
	c.retriableErrors[err] = strategy
	return c
}

func (c Caller) WithRetryOnAnyError(strategy BackoffStrategy) Caller {
	c.anyErrorStrategy = &strategy
	return c
}

func (c Caller) WithMaxDuration(maxDuration time.Duration) Caller {
	c.maxDuration = &maxDuration
	return c
}

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
				return err
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
