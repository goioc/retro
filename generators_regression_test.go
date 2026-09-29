package retro

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrowingGeneratorsSaturate(t *testing.T) {
	for name, generator := range map[string]Generator{
		"linear":      NewLinear(math.MaxInt64 / 2),
		"exponential": NewExponential(2),
		"fibonacci":   NewFibonacci(),
	} {
		t.Run(name, func(t *testing.T) {
			previous := int64(0)
			for i := 0; i < 256; i++ {
				next := generator.Next()
				require.GreaterOrEqual(t, next, previous)
				previous = next
			}
			require.Equal(t, int64(math.MaxInt64), previous)
		})
	}
}

func TestExponentialUsesExactIntegerArithmetic(t *testing.T) {
	g := NewExponential(3)
	for i := 0; i < 34; i++ {
		g.Next()
	}
	require.Equal(t, int64(16677181699666569), g.Next()) // 3^34 is not exactly representable as float64.
}

func TestRandomValidatesBoundsAtConstruction(t *testing.T) {
	for _, max := range []int64{0, -1, math.MinInt64} {
		require.PanicsWithValue(t, "retro: random maximum must be positive", func() { NewRandom(max) })
	}
	require.Zero(t, NewRandom(1).Next())
}

func TestGeneratorsWithZeroAndNegativeParameters(t *testing.T) {
	for name, tc := range map[string]struct {
		generator Generator
		want      []int64
	}{
		"constant":     {NewConstant(-1), []int64{-1, -1, -1}},
		"linear zero":  {NewLinear(0), []int64{0, 0, 0}},
		"linear floor": {NewLinear(math.MinInt64), []int64{0, math.MinInt64, math.MinInt64}},
		"exp zero":     {NewExponential(0), []int64{1, 0, 0}},
		"exp one":      {NewExponential(1), []int64{1, 1, 1}},
		"exp negative": {NewExponential(-2), []int64{1, -2, 4}},
	} {
		t.Run(name, func(t *testing.T) {
			for _, want := range tc.want {
				require.Equal(t, want, tc.generator.Next())
			}
		})
	}
}

func FuzzSaturatingArithmetic(f *testing.F) {
	for _, a := range []int64{math.MinInt64, math.MinInt64 + 1, -2, -1, 0, 1, 2, math.MaxInt64 - 1, math.MaxInt64} {
		for _, b := range []int64{math.MinInt64, -2, -1, 0, 1, 2, math.MaxInt64} {
			f.Add(a, b)
		}
	}
	f.Fuzz(func(t *testing.T, a, b int64) {
		clamp := func(n *big.Int) int64 {
			if n.Cmp(big.NewInt(math.MaxInt64)) > 0 {
				return math.MaxInt64
			}
			if n.Cmp(big.NewInt(math.MinInt64)) < 0 {
				return math.MinInt64
			}
			return n.Int64()
		}
		require.Equal(t, clamp(new(big.Int).Add(big.NewInt(a), big.NewInt(b))), saturatingAdd(a, b))
		require.Equal(t, clamp(new(big.Int).Mul(big.NewInt(a), big.NewInt(b))), saturatingMultiply(a, b))
	})
}
