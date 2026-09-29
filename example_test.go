package retro_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/goioc/retro"
)

func ExampleCaller_Call() {
	caller := retro.NewCaller().WithRetryOnAnyError(
		retro.NewBackoffStrategy(retro.NewConstant(1), 100*time.Millisecond).
			WithMaxRetries(3),
	)

	attempts := 0
	err := caller.Call(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	fmt.Printf("attempts=%d, err=%v\n", attempts, err)
	// Output: attempts=3, err=<nil>
}

func ExampleCaller_WithRetriableError() {
	notReady := errors.New("not ready")
	busy := errors.New("busy")

	caller := retro.NewCaller().
		WithRetriableError(notReady,
			retro.NewBackoffStrategy(retro.NewConstant(1), 100*time.Millisecond).
				WithMaxRetries(3)).
		WithRetriableError(busy,
			retro.NewBackoffStrategy(retro.NewExponential(2), 10*time.Millisecond).
				WithCappedDuration(time.Second).
				WithMaxRetries(5))

	attempts := 0
	err := caller.Call(context.Background(), func() error {
		attempts++
		switch attempts {
		case 1:
			return fmt.Errorf("lookup: %w", notReady)
		case 2:
			return busy
		default:
			return nil
		}
	})

	fmt.Printf("attempts=%d, err=%v\n", attempts, err)
	// Output: attempts=3, err=<nil>
}

func ExampleCaller_Call_cancellation() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	caller := retro.NewCaller().WithRetryOnAnyError(
		retro.NewBackoffStrategy(retro.NewConstant(1), time.Second).
			WithMaxRetries(3),
	)
	temporary := errors.New("temporary failure")
	err := caller.Call(ctx, func() error {
		cancel()
		return temporary
	})

	fmt.Println("canceled:", errors.Is(err, context.Canceled))
	fmt.Println("operation error:", errors.Is(err, temporary))
	// Output:
	// canceled: true
	// operation error: true
}

// schedule repeats its final multiplier after producing 1, 2, and 5.
type schedule struct{ index int }

// Next advances through the schedule, then keeps returning its final value.
func (g *schedule) Next() int64 {
	steps := [...]int64{1, 2, 5}
	value := steps[g.index]
	if g.index < len(steps)-1 {
		g.index++
	}
	return value
}

func ExampleNewBackoffStrategyWithFactory() {
	strategy := retro.NewBackoffStrategyWithFactory(
		func() retro.Generator { return &schedule{} },
		100*time.Millisecond,
	).WithMaxRetries(3)

	for {
		delay, err := strategy.Delay()
		if err != nil {
			fmt.Println("exhausted:", errors.Is(err, retro.ErrMaxRetries))
			break
		}
		fmt.Println(delay)
	}
	// Output:
	// 100ms
	// 200ms
	// 500ms
	// exhausted: true
}
