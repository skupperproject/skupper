package utils

import (
	"context"
	"time"

	"github.com/briandowns/spinner"
	"github.com/skupperproject/skupper/pkg/utils"
)

func NewSpinner(message string, maxRetries int, function func() error) error {

	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond, spinner.WithHiddenCursor(false))
	defer spin.Stop()
	spin.Prefix = message
	spin.FinalMSG = message + "\n"

	spin.Start()

	err := utils.RetryError(time.Second, maxRetries, function)

	spin.Stop()

	if err != nil {
		return err
	}

	return nil
}

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
