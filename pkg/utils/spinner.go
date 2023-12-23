package utils

import (
	"github.com/briandowns/spinner"
	"golang.org/x/sys/unix"
	"os"
	"syscall"
	"time"
)

func NewSpinner(message string, maxRetries int, function func() error) error {
	spin := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	spin.Prefix = message
	spin.FinalMSG = message + "\n"

	if IsRunningInForegroundOrDefault() {
		spin.Start()
	}

	err := RetryError(time.Second, maxRetries, function)

	spin.Stop()

	if err != nil {
		return err
	}

	return nil
}

func IsRunningInForegroundOrDefault() bool {
	// Get the foreground process group ID of the terminal
	foregroundPGID, err := unix.IoctlGetInt(int(os.Stdout.Fd()), unix.TIOCGPGRP)
	if err != nil {
		// spinner running by default
		return true
	}

	// Get the process group ID of the current process
	currentPGID := syscall.Getpgrp()

	// Check if the process is in the foreground
	return currentPGID == foregroundPGID
}
