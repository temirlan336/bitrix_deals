package syncer

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

func callWithRetry(ctx context.Context, attempts int, fn func(context.Context) error) error {
	if attempts < 1 {
		attempts = 1
	}

	var last error
	backoff := 400 * time.Millisecond

	for i := 1; i <= attempts; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		last = err

		if !isTransient(err) {
			return err
		}

		time.Sleep(backoff)
		backoff *= 2
	}

	return last
}

func isTransient(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout() || ne.Temporary()
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "context deadline exceeded")
}
