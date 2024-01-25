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

// Generator interface describes an abstraction for implementing sequence generators (used in the back-off strategy).
type Generator interface {
	// Next method returns a next number in the sequence.
	Next() int64
}

type random struct {
	max int64
}

// NewRandom creates a Generator based on math.rand, i.e. all the numbers are random (within a specified interval from 0
// to `max`).
func NewRandom(max int64) Generator {
	return random{max: max}
}

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

func (g constant) Next() int64 {
	return g.c
}

type linear struct {
	iteration int64
	delta     int64
}

// NewLinear creates a Generator where every next number is the previous number plus some `delta` (starts with 0).
func NewLinear(delta int64) Generator {
	return &linear{delta: delta}
}

func (g *linear) Next() int64 {
	defer func() {
		g.iteration++
	}()
	return g.iteration * g.delta
}

type exponential struct {
	iteration int64
	factor    int64
}

// NewExponential creates a Generator where every next number is the previous number multiplied by some `factor` (starts
// with 0).
func NewExponential(factor int64) Generator {
	return &exponential{factor: factor}
}

func (g *exponential) Next() int64 {
	defer func() {
		g.iteration++
	}()
	return int64(math.Pow(float64(g.factor), float64(g.iteration)))
}

type fibonacci struct {
	prev int64
	cur  int64
}

// NewFibonacci creates a Generator where every next number is a sum of two previous numbers (Fibonacci sequence).
// Starts with 1.
func NewFibonacci() Generator {
	return &fibonacci{
		prev: 0,
		cur:  1,
	}
}

func (g *fibonacci) Next() int64 {
	defer func() {
		cur := g.cur
		g.cur = g.prev + g.cur
		g.prev = cur
	}()
	return g.cur
}
