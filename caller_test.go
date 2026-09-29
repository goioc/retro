package retro

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	err1 = errors.New("err1")
)

type tester struct {
	mock.Mock
}

func (t *tester) bar() error {
	return t.Called().Error(0)
}

func TestCaller(t *testing.T) {
	foo := tester{}
	foo.On("bar").Return(err1)

	caller := NewCaller()
	err := caller.Call(context.Background(), foo.bar)
	require.ErrorIs(t, err, err1)
	foo.AssertNumberOfCalls(t, "bar", 1)
}

func TestCallerWithRetriableError(t *testing.T) {
	foo := tester{}
	foo.On("bar").Return(err1)

	caller := NewCaller().WithRetriableError(err1, NewBackoffStrategy(NewConstant(10), time.Millisecond).WithMaxRetries(10))
	err := caller.Call(context.Background(), foo.bar)
	require.ErrorIs(t, err, err1)
	foo.AssertNumberOfCalls(t, "bar", 11)
}

func TestCallerWithRetryOnAnyError(t *testing.T) {
	foo := tester{}
	foo.On("bar").Return(err1)

	caller := NewCaller().WithRetryOnAnyError(NewBackoffStrategy(NewConstant(10), time.Millisecond).WithMaxRetries(10))
	err := caller.Call(context.Background(), foo.bar)
	require.ErrorIs(t, err, err1)
	foo.AssertNumberOfCalls(t, "bar", 11)
}

func TestCallerWithMaxDuration(t *testing.T) {
	caller := NewCaller().WithRetryOnAnyError(NewBackoffStrategy(NewConstant(1), time.Hour)).WithMaxDuration(20 * time.Millisecond)
	now := time.Now()
	err := caller.Call(context.Background(), func() error {
		return err1
	})
	require.ErrorIs(t, err, err1)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(now), time.Second)
}
