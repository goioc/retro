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
	"math"
	"testing"
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

func fib(n int64) int64 {
	if n <= 1 {
		return n
	}
	return fib(n-1) + fib(n-2)
}
