package utils

import (
	"time"

	"github.com/briandowns/spinner"
)

func NewSpinner(message string, maxRetries int, function func() error) error {

	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond, spinner.WithHiddenCursor(false))
	spin.Prefix = message
	spin.FinalMSG = message + "\n"

	spin.Start()

	err := RetryError(time.Second, maxRetries, function)

	spin.Stop()

	if err != nil {
		return err
	}

	return nil
}
