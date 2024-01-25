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
	"k8s.io/utils/pointer"
	"math"
	"math/rand"
	"time"
)

type BackoffStrategy struct {
	generator                   Generator
	durationUnit                time.Duration
	jitterInNanoseconds         int64
	cappedDurationInNanoseconds int64
	maxRetries                  int64

	called *int64
}

func NewBackoffStrategy(generator Generator, durationUnit time.Duration) BackoffStrategy {
	return BackoffStrategy{
		generator:                   generator,
		durationUnit:                durationUnit,
		cappedDurationInNanoseconds: math.MaxInt64,
		maxRetries:                  math.MaxInt64,
		called:                      pointer.Int64(0),
	}
}

func (b BackoffStrategy) WithJitter(jitter time.Duration) BackoffStrategy {
	b.jitterInNanoseconds = jitter.Nanoseconds()
	return b
}

func (b BackoffStrategy) WithCappedDuration(cappedDuration time.Duration) BackoffStrategy {
	b.cappedDurationInNanoseconds = cappedDuration.Nanoseconds()
	return b
}

func (b BackoffStrategy) WithMaxRetries(maxRetries int64) BackoffStrategy {
	b.maxRetries = maxRetries
	return b
}

func (b BackoffStrategy) Delay() (time.Duration, error) {
	*b.called++
	if *b.called >= b.maxRetries {
		return time.Duration(0), errors.New("reached max retries")
	}
	step := b.generator.Next()
	durationInNanoseconds := step * b.durationUnit.Nanoseconds()
	jitterInNanoseconds := int64(0)
	if b.jitterInNanoseconds != 0 {
		jitterInNanoseconds = rand.Int63n(2*b.jitterInNanoseconds) - b.jitterInNanoseconds
	}
	durationInNanoseconds += jitterInNanoseconds
	if durationInNanoseconds < 0 {
		durationInNanoseconds = 0
	} else if durationInNanoseconds > b.cappedDurationInNanoseconds {
		durationInNanoseconds = b.cappedDurationInNanoseconds
	}
	return time.Duration(durationInNanoseconds), nil
}
