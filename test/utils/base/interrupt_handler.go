package base

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// HandleInterruptSignal runs the given fn in case
// test execution was interrupted
func HandleInterruptSignal(fn func()) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChannel
		log.Printf("interrupt signal received")
		fn()
	}()
}
