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
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBackoffStrategy(t *testing.T) {
	strategy := NewBackoffStrategy(NewConstant(10), time.Millisecond)
	delay, err := strategy.Delay()
	require.NoError(t, err)
	require.Equal(t, 10*time.Millisecond, delay)
}

func TestBackoffStrategyWithJitter(t *testing.T) {
	strategy := NewBackoffStrategy(NewConstant(10), time.Millisecond).WithJitter(5 * time.Millisecond)
	for i := 0; i < 10; i++ {
		delay, err := strategy.Delay()
		require.NoError(t, err)
		require.GreaterOrEqual(t, delay, 5*time.Millisecond)
		require.LessOrEqual(t, delay, 15*time.Millisecond)
	}
}

func TestBackoffStrategyWithCappedDuration(t *testing.T) {
	strategy := NewBackoffStrategy(NewLinear(10), time.Millisecond).WithCappedDuration(50 * time.Millisecond)
	for i := 0; i < 10; i++ {
		expected := i * 10
		if expected > 50 {
			expected = 50
		}
		delay, err := strategy.Delay()
		require.NoError(t, err)
		require.Equal(t, time.Millisecond*time.Duration(expected), delay)
	}
}

func TestBackoffStrategyWithMaxRetries(t *testing.T) {
	strategy := NewBackoffStrategy(NewConstant(10), time.Millisecond).WithMaxRetries(5)
	for i := 0; i < 10; i++ {
		_, err := strategy.Delay()
		if i < 4 { // one less, because it's applied after the function call
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, "reached max retries")
		}
	}
}
