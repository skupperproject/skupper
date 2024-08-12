package utils

import (
	"github.com/briandowns/spinner"
	"time"
)

func NewSpinner(message string, maxRetries int, function func() error) error {

	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	defer spin.Stop()
	spin.Prefix = message
	spin.FinalMSG = message + "\n"

	spin.Start()

	err := RetryError(time.Second, maxRetries, function)

	if err != nil {
		return err
	}

	return nil
}
