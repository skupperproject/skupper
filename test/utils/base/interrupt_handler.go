package base

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// HandleInterruptSignal runs the given fn in case
// test execution was interrupted
func HandleInterruptSignal(t *testing.T, fn func(*testing.T)) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChannel
		t.Logf("interrupt signal received")
		fn(t)
	}()
}
