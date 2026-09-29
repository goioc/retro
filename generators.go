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
	"math"
	"math/rand"
)

// Generator produces a backoff sequence. Stateful generators should not be
// shared between goroutines. Caller creates its own generators for each Call.
type Generator interface {
	// Next method returns a next number in the sequence.
	Next() int64
}

type random struct {
	max int64
}

// NewRandom creates a Generator producing values in [0, max).
// It panics immediately if max is not positive, like rand.Int63n.
func NewRandom(max int64) Generator {
	if max <= 0 {
		panic("retro: random maximum must be positive")
	}
	return random{max: max}
}

func (g random) newGenerator() Generator { return NewRandom(g.max) }

func (g random) Next() int64 {
	return rand.Int63n(g.max)
}

type constant struct {
	c int64
}

// NewConstant creates a Generator that always return the same number (specified as `c` parameter).
func NewConstant(c int64) Generator {
	return constant{c: c}
}

func (g constant) newGenerator() Generator { return NewConstant(g.c) }

func (g constant) Next() int64 {
	return g.c
}

type linear struct {
	value int64
	delta int64
}

// NewLinear starts at 0 and adds delta each time. Values saturate at the int64
// bounds instead of wrapping on overflow.
func NewLinear(delta int64) Generator {
	return &linear{delta: delta}
}

func (g *linear) Next() int64 {
	value := g.value
	g.value = saturatingAdd(g.value, g.delta)
	return value
}

func (g *linear) newGenerator() Generator { return NewLinear(g.delta) }

type exponential struct {
	value  int64
	factor int64
}

// NewExponential starts at 1 and multiplies by factor each time. Integer
// arithmetic preserves exact values and saturates at the int64 bounds.
func NewExponential(factor int64) Generator {
	return &exponential{value: 1, factor: factor}
}

func (g *exponential) Next() int64 {
	value := g.value
	g.value = saturatingMultiply(g.value, g.factor)
	return value
}

func (g *exponential) newGenerator() Generator { return NewExponential(g.factor) }

type fibonacci struct {
	prev int64
	cur  int64
}

// NewFibonacci creates a Generator where every next number is a sum of two previous numbers (Fibonacci sequence).
// Starts with 1 and saturates at math.MaxInt64 on overflow.
func NewFibonacci() Generator {
	return &fibonacci{
		prev: 0,
		cur:  1,
	}
}

func (g *fibonacci) Next() int64 {
	value := g.cur
	g.prev, g.cur = g.cur, saturatingAdd(g.prev, g.cur)
	return value
}

func (g *fibonacci) newGenerator() Generator { return NewFibonacci() }

func saturatingAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

func saturatingMultiply(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
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
