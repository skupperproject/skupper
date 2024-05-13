package utils

import (
	"github.com/briandowns/spinner"
	"github.com/skupperproject/skupper/pkg/utils"
	"time"
)

func NewSpinner(message string, maxRetries int, function func() error) error {

	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
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
