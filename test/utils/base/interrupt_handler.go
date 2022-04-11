package base

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// This indicates that an interrupt signal has been received at least
// once.  Functions can access it directly to check whether they should
// continue, or call StopIfInterrupted (if they have t *testing.T)
var UserInterrupted bool

// HandleInterruptSignal runs the given fn in case
// test execution was interrupted
func HandleInterruptSignal(fn func()) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChannel
		UserInterrupted = true
		log.Printf("interrupt signal received")
		fn()
	}()
}

// Calls *testing.T.Fatalf if base.UserInterrupted is true
// In other words, stop that test if someone hit Ctrl+C
func StopIfInterrupted(t *testing.T) {
	if UserInterrupted {
		t.Fatalf("Stopping test as user hit interrupt")
	}
}
