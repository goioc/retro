package retro

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCallerMatchesWrappedAndJoinedErrors(t *testing.T) {
	sentinel := errors.New("transient")
	for name, firstErr := range map[string]error{
		"wrapped": fmt.Errorf("query: %w", sentinel),
		"joined":  errors.Join(errors.New("other"), sentinel),
	} {
		t.Run(name, func(t *testing.T) {
			caller := NewCaller().WithRetriableError(sentinel,
				NewBackoffStrategy(NewConstant(0), 0).WithMaxRetries(1))
			calls := 0
			err := caller.Call(context.Background(), func() error {
				calls++
				if calls == 1 {
					return firstErr
				}
				return nil
			})
			require.NoError(t, err)
			require.Equal(t, 2, calls)
		})
	}
}

// sliceError exercises error matching with a value that cannot be a map key.
type sliceError []string

func (e sliceError) Error() string { return e[0] }
func (e sliceError) Is(target error) bool {
	other, ok := target.(sliceError)
	return ok && e[0] == other[0]
}

func TestCallerAcceptsNonComparableErrors(t *testing.T) {
	caller := NewCaller().WithRetriableError(sliceError{"transient"},
		NewBackoffStrategy(NewConstant(0), 0).WithMaxRetries(1))
	calls := 0
	err := caller.Call(context.Background(), func() error {
		calls++
		if calls == 1 {
			return fmt.Errorf("wrapped: %w", sliceError{"transient"})
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

func TestCallerRetryBudgetsResetForEveryCall(t *testing.T) {
	for _, limit := range []int64{0, 1, 3} {
		for _, anyError := range []bool{false, true} {
			t.Run(fmt.Sprintf("limit=%d/any=%v", limit, anyError), func(t *testing.T) {
				strategy := NewBackoffStrategy(NewLinear(1), 0).WithMaxRetries(limit)
				caller := NewCaller().WithRetriableError(err1, strategy)
				if anyError {
					caller = NewCaller().WithRetryOnAnyError(strategy)
				}
				for run := 0; run < 2; run++ {
					calls := 0
					err := caller.Call(context.Background(), func() error { calls++; return err1 })
					require.ErrorIs(t, err, err1)
					require.EqualValues(t, limit+1, calls)
				}
			})
		}
	}
}

func TestCallerRulesHaveIndependentBudgets(t *testing.T) {
	err2 := errors.New("second")
	strategy := NewBackoffStrategy(NewConstant(0), 0).WithMaxRetries(1)
	caller := NewCaller().WithRetriableError(err1, strategy).WithRetriableError(err2, strategy)
	results := []error{err1, err2, nil}
	calls := 0
	err := caller.Call(context.Background(), func() error {
		result := results[calls]
		calls++
		return result
	})
	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestCallerConfigurationIsIndependent(t *testing.T) {
	err2, err3 := errors.New("second"), errors.New("third")
	strategy := NewBackoffStrategy(NewConstant(0), 0).WithMaxRetries(1)
	base := NewCaller().WithRetriableError(err1, strategy)
	second := base.WithRetriableError(err2, strategy)
	third := base.WithRetriableError(err3, strategy)
	for name, tc := range map[string]struct {
		caller Caller
		calls  int
	}{
		"base":   {base, 1},
		"second": {second, 2},
		"third":  {third, 1},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			err := tc.caller.Call(context.Background(), func() error { calls++; return err2 })
			require.ErrorIs(t, err, err2)
			require.Equal(t, tc.calls, calls)
		})
	}
}

func TestCallerUsesFirstMatchingRule(t *testing.T) {
	err2 := errors.New("second")
	joined := errors.Join(err1, err2)
	noRetry := NewBackoffStrategy(NewConstant(0), 0).WithMaxRetries(0)
	oneRetry := NewBackoffStrategy(NewConstant(0), 0).WithMaxRetries(1)
	for name, tc := range map[string]struct {
		caller Caller
		calls  int
	}{
		"first rule": {NewCaller().WithRetriableError(err1, noRetry).WithRetriableError(err2, oneRetry), 1},
		"reversed":   {NewCaller().WithRetriableError(err2, oneRetry).WithRetriableError(err1, noRetry), 2},
		"any error":  {NewCaller().WithRetriableError(err1, oneRetry).WithRetryOnAnyError(noRetry), 1},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			_ = tc.caller.Call(context.Background(), func() error { calls++; return joined })
			require.Equal(t, tc.calls, calls)
		})
	}
}

func TestCallerDoesNotExecuteWithExpiredContext(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, release := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer release()
	for name, tc := range map[string]struct {
		ctx    context.Context
		caller Caller
		want   error
	}{
		"canceled":     {canceled, NewCaller(), context.Canceled},
		"expired":      {expired, NewCaller(), context.DeadlineExceeded},
		"zero timeout": {context.Background(), NewCaller().WithMaxDuration(0), context.DeadlineExceeded},
		"past timeout": {context.Background(), NewCaller().WithMaxDuration(-time.Second), context.DeadlineExceeded},
	} {
		t.Run(name, func(t *testing.T) {
			err := tc.caller.Call(tc.ctx, func() error { t.Fatal("unexpected callback"); return nil })
			require.ErrorIs(t, err, tc.want)
		})
	}
}

// notifyingGenerator signals when backoff begins so cancellation tests do not
// depend on sleeps. Each instance is used for a single retry.
type notifyingGenerator struct{ started chan struct{} }

func (g *notifyingGenerator) Next() int64 {
	close(g.started)
	return 1
}

func TestCallerCancellationInterruptsBackoff(t *testing.T) {
	for _, anyError := range []bool{false, true} {
		t.Run(fmt.Sprintf("any=%v", anyError), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			strategy := NewBackoffStrategyWithFactory(func() Generator {
				return &notifyingGenerator{started: started}
			}, time.Hour)
			caller := NewCaller().WithRetriableError(err1, strategy)
			if anyError {
				caller = NewCaller().WithRetryOnAnyError(strategy)
			}
			result := make(chan error, 1)
			go func() { result <- caller.Call(ctx, func() error { return err1 }) }()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("retry did not start")
			}
			cancel()
			select {
			case err := <-result:
				require.ErrorIs(t, err, context.Canceled)
				require.ErrorIs(t, err, err1)
			case <-time.After(time.Second):
				t.Fatal("cancellation did not interrupt backoff")
			}
		})
	}
}

func TestCallerCancellationDuringCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	caller := NewCaller().WithRetryOnAnyError(NewBackoffStrategy(NewConstant(1), time.Hour))
	err := caller.Call(ctx, func() error { cancel(); return err1 })
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, err1)
}

func TestCallerConcurrentCalls(t *testing.T) {
	for _, anyError := range []bool{false, true} {
		t.Run(fmt.Sprintf("any=%v", anyError), func(t *testing.T) {
			strategy := NewBackoffStrategy(NewLinear(1), 0).WithMaxRetries(20)
			caller := NewCaller().WithRetriableError(err1, strategy)
			if anyError {
				caller = NewCaller().WithRetryOnAnyError(strategy)
			}
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := 0; i < 16; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					calls := 0
					err := caller.Call(context.Background(), func() error {
						calls++
						if calls <= 20 {
							return err1
						}
						return nil
					})
					if err != nil || calls != 21 {
						t.Errorf("got %d attempts, error %v; want success on attempt 21", calls, err)
					}
				}()
			}
			close(start)
			wg.Wait()
		})
	}
}

// recordingGenerator records a sequence to expose accidental state sharing.
type recordingGenerator struct{ values []int64 }

func (g *recordingGenerator) Next() int64 {
	next := int64(len(g.values))
	g.values = append(g.values, next)
	return next
}

func TestCallerCreatesIndependentCustomGenerators(t *testing.T) {
	created := make(chan *recordingGenerator, 16)
	strategy := NewBackoffStrategyWithFactory(func() Generator {
		generator := &recordingGenerator{}
		created <- generator
		return generator
	}, 0).WithMaxRetries(2)
	caller := NewCaller().WithRetryOnAnyError(strategy)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = caller.Call(context.Background(), func() error { return err1 })
		}()
	}
	wg.Wait()
	close(created)
	require.Len(t, created, 16)
	for generator := range created {
		require.Equal(t, []int64{0, 1}, generator.values)
	}
}

func TestCallerDoesNotConsumeDirectDelayState(t *testing.T) {
	strategy := NewBackoffStrategy(NewLinear(1), time.Nanosecond).WithMaxRetries(2)
	delay, err := strategy.Delay()
	require.NoError(t, err)
	require.Zero(t, delay)
	caller := NewCaller().WithRetryOnAnyError(strategy)
	for i := 0; i < 2; i++ {
		calls := 0
		_ = caller.Call(context.Background(), func() error { calls++; return err1 })
		require.Equal(t, 3, calls)
	}
	delay, err = strategy.Delay()
	require.NoError(t, err)
	require.Equal(t, time.Nanosecond, delay)
}

func TestCallerReportsConfigurationErrors(t *testing.T) {
	for name, strategy := range map[string]BackoffStrategy{
		"custom without factory": NewBackoffStrategy(&recordingGenerator{}, 0),
		"negative jitter":        NewBackoffStrategy(NewConstant(0), 0).WithJitter(-1),
		"nil factory result":     NewBackoffStrategyWithFactory(func() Generator { return nil }, 0),
	} {
		t.Run(name, func(t *testing.T) {
			caller := NewCaller().WithRetryOnAnyError(strategy)
			err := caller.Call(context.Background(), func() error { return err1 })
			require.ErrorIs(t, err, err1)
			require.ErrorContains(t, err, "retro:")
			require.NoError(t, caller.Call(context.Background(), func() error { return nil }))
		})
	}
}
