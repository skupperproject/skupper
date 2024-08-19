package utils

import (
	"context"
	"github.com/briandowns/spinner"
	"os"
	"os/signal"
	"time"
)

func NewSpinner(message string, maxRetries int, function func() error) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	go func() {
		<-signals
		cancel()
	}()

	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	defer spin.Stop()
	spin.Prefix = message
	spin.FinalMSG = message + "\n"

	spin.Start()

	defer cancel()

	err := TryUntilWithContext(ctx, time.Second, maxRetries, function)

	if err != nil {
		return err
	}

	return nil
}
