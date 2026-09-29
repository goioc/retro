package retro

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomGenerator(t *testing.T) {
	generator := NewRandom(10)
	for i := 0; i < 10; i++ {
		next := generator.Next()
		require.GreaterOrEqual(t, next, int64(0))
		require.Less(t, next, int64(10))
	}
}

func TestConstantGenerator(t *testing.T) {
	generator := NewConstant(10)
	for i := 0; i < 10; i++ {
		require.Equal(t, int64(10), generator.Next())
	}
}

func TestLinearGenerator(t *testing.T) {
	generator := NewLinear(10)
	for i := 0; i < 10; i++ {
		require.Equal(t, int64(i*10), generator.Next())
	}
}

func TestExponentialGenerator(t *testing.T) {
	generator := NewExponential(10)
	for i := 0; i < 10; i++ {
		require.Equal(t, int64(math.Pow(10, float64(i))), generator.Next())
	}
}

func TestFibonacciGenerator(t *testing.T) {
	generator := NewFibonacci()
	for i := 0; i < 10; i++ {
		require.Equal(t, fib(int64(i+1)), generator.Next())
	}
}

// fib is a recursive reference used to check the stateful Fibonacci generator.
func fib(n int64) int64 {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}
