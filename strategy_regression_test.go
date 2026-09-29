package retro

import (
	"math"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCappedBackoffDoesNotOverflow(t *testing.T) {
	for name, generator := range map[string]Generator{
		"exponential": NewExponential(2),
		"fibonacci":   NewFibonacci(),
		"linear":      NewLinear(math.MaxInt64 / 2),
	} {
		t.Run(name, func(t *testing.T) {
			strategy := NewBackoffStrategy(generator, time.Millisecond).WithCappedDuration(time.Second)
			previous := time.Duration(0)
			for i := 0; i < 256; i++ {
				delay, err := strategy.Delay()
				require.NoError(t, err)
				require.GreaterOrEqual(t, delay, previous, "step %d", i)
				require.LessOrEqual(t, delay, time.Second)
				previous = delay
			}
			require.Equal(t, time.Second, previous)
		})
	}
}

func TestDelaySaturatesDurationAndJitter(t *testing.T) {
	strategy := NewBackoffStrategy(NewConstant(math.MaxInt64), time.Second)
	delay, err := strategy.Delay()
	require.NoError(t, err)
	require.Equal(t, time.Duration(math.MaxInt64), delay)

	withJitter := strategy.WithJitter(time.Millisecond)
	for i := 0; i < 100; i++ {
		delay, err := withJitter.Delay()
		require.NoError(t, err)
		require.GreaterOrEqual(t, delay, time.Duration(math.MaxInt64)-time.Millisecond)
	}

	fullRange := NewBackoffStrategy(NewConstant(1), time.Second).
		WithJitter(time.Duration(math.MaxInt64)).WithCappedDuration(time.Second)
	for i := 0; i < 100; i++ {
		delay, err := fullRange.Delay()
		require.NoError(t, err)
		require.GreaterOrEqual(t, delay, time.Duration(0))
		require.LessOrEqual(t, delay, time.Second)
	}
}

func TestDelayRejectsInvalidConfiguration(t *testing.T) {
	valid := NewBackoffStrategy(NewConstant(1), time.Nanosecond)
	for name, strategy := range map[string]BackoffStrategy{
		"zero value":      {},
		"nil generator":   NewBackoffStrategy(nil, 0),
		"nil factory":     NewBackoffStrategyWithFactory(nil, 0),
		"nil result":      NewBackoffStrategyWithFactory(func() Generator { return nil }, 0),
		"negative unit":   NewBackoffStrategy(NewConstant(1), -1),
		"negative jitter": valid.WithJitter(-1),
		"negative cap":    valid.WithCappedDuration(-1),
		"negative limit":  valid.WithMaxRetries(-1),
	} {
		t.Run(name, func(t *testing.T) {
			delay, err := strategy.Delay()
			require.Error(t, err)
			require.Zero(t, delay)
		})
	}
}

func TestDelayClampsNegativeSequences(t *testing.T) {
	strategy := NewBackoffStrategy(NewConstant(math.MinInt64), time.Second)
	delay, err := strategy.Delay()
	require.NoError(t, err)
	require.Zero(t, delay)
}

func TestCustomGeneratorSupportsDirectDelay(t *testing.T) {
	strategy := NewBackoffStrategy(&recordingGenerator{}, time.Nanosecond)
	for i := 0; i < 3; i++ {
		delay, err := strategy.Delay()
		require.NoError(t, err)
		require.Equal(t, time.Duration(i), delay)
	}
}

func TestConcurrentDelaySharesOneSequence(t *testing.T) {
	strategy := NewBackoffStrategy(NewLinear(1), time.Nanosecond).WithMaxRetries(64)
	results := make(chan time.Duration, 64)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(copy BackoffStrategy) {
			defer wg.Done()
			delay, err := copy.Delay()
			if err != nil {
				t.Error(err)
				return
			}
			results <- delay
		}(strategy)
	}
	wg.Wait()
	close(results)
	var delays []time.Duration
	for delay := range results {
		delays = append(delays, delay)
	}
	sort.Slice(delays, func(i, j int) bool { return delays[i] < delays[j] })
	require.Len(t, delays, 64)
	for i, delay := range delays {
		require.Equal(t, time.Duration(i), delay)
	}
	_, err := strategy.Delay()
	require.ErrorIs(t, err, ErrMaxRetries)
}
