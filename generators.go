package retro

import (
	"math"
	"math/rand"
)

// Generator produces values that a [BackoffStrategy] multiplies by a duration
// unit. Stateful generators should not be shared between goroutines. Caller
// creates independent generators for each Call; custom generators used with
// Caller must be supplied through [NewBackoffStrategyWithFactory].
type Generator interface {
	// Next returns the next value and advances the sequence, if it has state.
	Next() int64
}

type random struct {
	max int64
}

// NewRandom returns a generator producing uniformly distributed values in
// [0, max). It panics if max is not positive, like [rand.Int63n].
func NewRandom(max int64) Generator {
	if max <= 0 {
		panic("retro: random maximum must be positive")
	}
	return random{max: max}
}

// newGenerator preserves the bound when creating a generator for a new Call.
func (g random) newGenerator() Generator { return NewRandom(g.max) }

// Next returns a random value in [0, g.max).
func (g random) Next() int64 {
	return rand.Int63n(g.max)
}

type constant struct {
	c int64
}

// NewConstant returns a generator that always produces c.
func NewConstant(c int64) Generator {
	return constant{c: c}
}

// newGenerator preserves the constant when creating a generator for a new Call.
func (g constant) newGenerator() Generator { return NewConstant(g.c) }

// Next returns the configured constant.
func (g constant) Next() int64 {
	return g.c
}

type linear struct {
	value int64
	delta int64
}

// NewLinear returns a generator producing 0, delta, 2*delta, and so on.
// The first value is zero, allowing an immediate first retry without jitter.
// Values saturate at the int64 bounds instead of wrapping on overflow.
func NewLinear(delta int64) Generator {
	return &linear{delta: delta}
}

// Next returns the current value and advances it by delta, saturating on overflow.
func (g *linear) Next() int64 {
	value := g.value
	g.value = saturatingAdd(g.value, g.delta)
	return value
}

// newGenerator restarts the sequence at zero with the same delta.
func (g *linear) newGenerator() Generator { return NewLinear(g.delta) }

type exponential struct {
	value  int64
	factor int64
}

// NewExponential returns a generator producing 1, factor, factor*factor, and so
// on. Integer arithmetic preserves exact values and saturates at the int64
// bounds instead of wrapping on overflow.
func NewExponential(factor int64) Generator {
	return &exponential{value: 1, factor: factor}
}

// Next returns the current value and multiplies it by factor for the next call.
func (g *exponential) Next() int64 {
	value := g.value
	g.value = saturatingMultiply(g.value, g.factor)
	return value
}

// newGenerator restarts the sequence at one with the same factor.
func (g *exponential) newGenerator() Generator { return NewExponential(g.factor) }

type fibonacci struct {
	prev int64
	cur  int64
}

// NewFibonacci returns a generator producing 1, 1, 2, 3, 5, and so on.
// Values saturate at [math.MaxInt64] instead of wrapping on overflow.
func NewFibonacci() Generator {
	return &fibonacci{
		prev: 0,
		cur:  1,
	}
}

// Next returns the current Fibonacci number and advances the pair of terms.
func (g *fibonacci) Next() int64 {
	value := g.cur
	g.prev, g.cur = g.cur, saturatingAdd(g.prev, g.cur)
	return value
}

// newGenerator restarts the Fibonacci sequence at one.
func (g *fibonacci) newGenerator() Generator { return NewFibonacci() }

// saturatingAdd returns a+b, clamped to the int64 bounds on overflow.
func saturatingAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

// saturatingMultiply returns a*b, clamped to the int64 bounds on overflow.
func saturatingMultiply(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	// Negating MinInt64 cannot be represented as an int64.
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return math.MaxInt64
	}
	product := a * b
	if product/b != a {
		if (a < 0) == (b < 0) {
			return math.MaxInt64
		}
		return math.MinInt64
	}
	return product
}
