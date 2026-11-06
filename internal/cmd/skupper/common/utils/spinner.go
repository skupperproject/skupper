package utils

import (
	"context"
	"time"

	"github.com/briandowns/spinner"
	"github.com/skupperproject/skupper/internal/utils"
)

func NewSpinnerWithTimeout(message string, timeoutInSeconds int, function func() error) error {

	retryProfile := GetConfiguredRetryProfile()
	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond, spinner.WithHiddenCursor(false))
	defer spin.Stop()

	spin.Prefix = message
	spin.FinalMSG = message + "\n"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(timeoutInSeconds))
	defer cancel()

	spin.Start()

	err := utils.RetryErrorWithContext(ctx, retryProfile.MinimumInterval, function)

	spin.Stop()

	if err != nil {
		return err
	}

	return nil
}
