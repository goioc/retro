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
	"context"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

import (
	"errors"
)

var (
	err1 = errors.New("err1")
)

type tester struct {
	mock.Mock
}

func (t *tester) bar() error {
	return t.Called().Get(0).(error)
}

func TestCaller(t *testing.T) {
	foo := tester{}
	foo.On("bar").Return(err1)

	caller := NewCaller()
	err := caller.Call(context.Background(), func() error {
		return foo.bar()
	})
	require.ErrorIs(t, err, err1)
	foo.AssertNumberOfCalls(t, "bar", 1)
}

func TestCallerWithRetriableError(t *testing.T) {
	foo := tester{}
	foo.On("bar").Return(err1)

	caller := NewCaller().WithRetriableError(err1, NewBackoffStrategy(NewConstant(10), time.Millisecond).WithMaxRetries(10))
	err := caller.Call(context.Background(), func() error {
		return foo.bar()
	})
	require.ErrorIs(t, err, err1)
	foo.AssertNumberOfCalls(t, "bar", 10)
}

func TestCallerWithRetryOnAnyError(t *testing.T) {
	foo := tester{}
	foo.On("bar").Return(err1)

	caller := NewCaller().WithRetryOnAnyError(NewBackoffStrategy(NewConstant(10), time.Millisecond).WithMaxRetries(10))
	err := caller.Call(context.Background(), func() error {
		return foo.bar()
	})
	require.ErrorIs(t, err, err1)
	foo.AssertNumberOfCalls(t, "bar", 10)
}

func TestCallerWithMaxDuration(t *testing.T) {
	caller := NewCaller().WithRetryOnAnyError(NewBackoffStrategy(NewConstant(10), time.Millisecond)).WithMaxDuration(time.Second)
	now := time.Now()
	err := caller.Call(context.Background(), func() error {
		return err1
	})
	elapsed := time.Since(now).Milliseconds()
	require.ErrorIs(t, err, err1)
	require.GreaterOrEqual(t, elapsed, int64(1000))
	require.Less(t, elapsed, int64(1100))
}
