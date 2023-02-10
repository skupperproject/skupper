package utils

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"gotest.tools/assert"
)

func TestTcpPortNextFree(t *testing.T) {
	minPort, err := TcpPortNextFree(1024)
	assert.Assert(t, err, "no available tcp ports found")

	ctx, cancel := context.WithCancel(context.Background())

	// listening on minPort to validate if it reports as in use
	wg := listenTcpPort(ctx, minPort)
	// waiting on port to be bound
	wg.Wait()

	// assert TcpPortNextFree shows a different port
	newMinPort, err := TcpPortNextFree(minPort)
	assert.Assert(t, err, "no more available tcp ports found")
	assert.Assert(t, newMinPort > minPort, "expected next free port available to be higher than %d but got %d", minPort, newMinPort)
	cancel()
}

func TestTcpPortInUse(t *testing.T) {
	minPort, err := TcpPortNextFree(1024)
	assert.Assert(t, err, "no available tcp ports found")

	ctx := context.Background()

	// listening on minPort to validate if it reports as in use
	wg := listenTcpPort(ctx, minPort)
	// waiting on port to be bound
	wg.Wait()

	// assert TcpPortInUse reports port as being used
	assert.Assert(t, TcpPortInUse("", minPort), "%d expected to be in use", minPort)

	// getting an extra port
	nextMinPort, err := TcpPortNextFree(minPort)
	assert.Assert(t, err, "no more available tcp ports found")
	assert.Assert(t, !TcpPortInUse("", nextMinPort), "tcp port %d expected to be available", nextMinPort)
}

func listenTcpPort(ctx context.Context, port int) *sync.WaitGroup {
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		wg.Done()
		if err != nil {
			<-ctx.Done()
			_ = listener.Close()
		}
	}()
	return wg
}
