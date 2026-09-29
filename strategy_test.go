package retro

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
		if i < 5 {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, ErrMaxRetries)
		}
	}
}
