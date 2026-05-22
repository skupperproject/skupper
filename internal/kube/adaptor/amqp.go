package adaptor

import (
	"context"
	"log/slog"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/skupperproject/skupper/internal/qdr"
)

const LocalRouterAMQPAddress = "amqp://localhost:5672"

// WaitForAMQPConnection retries until a connection to the local router AMQP
// bus succeeds or the context is cancelled.
func WaitForAMQPConnection(ctx context.Context, address string, maxInterval time.Duration) error {
	b := backoff.NewExponentialBackOff()
	b.MaxInterval = maxInterval
	b.MaxElapsedTime = 0
	b.Reset()

	pool := qdr.NewAgentPool(address, nil)
	pool.SetConnectionTimeout(maxInterval)
	return backoff.RetryNotify(
		func() error {
			agent, err := pool.Get()
			if err != nil {
				return err
			}
			agent.Close()
			slog.Info("Connected to router", slog.String("address", address))
			return nil
		},
		backoff.WithContext(b, ctx),
		func(err error, d time.Duration) {
			slog.Info("Waiting for AMQP connection",
				slog.String("address", address),
				slog.Any("error", err),
				slog.Duration("retryAfter", d),
			)
		},
	)
}
