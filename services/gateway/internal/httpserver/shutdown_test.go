package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

// freeAddress asks the kernel for an unused loopback port.
func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func TestRunDrainsInFlightRequests(t *testing.T) {
	requestStarted := make(chan struct{})
	slowHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		time.Sleep(300 * time.Millisecond)
		_, _ = responseWriter.Write([]byte("finished"))
	})

	address := freeAddress(t)
	server := NewServer(address, slowHandler, discardLogger())
	ctx, cancel := context.WithCancelCause(context.Background())
	runResult := make(chan error, 1)
	go func() { runResult <- Run(ctx, server, 5*time.Second, discardLogger()) }()

	statusCodes := make(chan int, 1)
	go func() {
		// Retry until the listener is up.
		for range 50 {
			response, err := http.Get("http://" + address + "/")
			if err == nil {
				response.Body.Close()
				statusCodes <- response.StatusCode
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		statusCodes <- 0
	}()

	<-requestStarted
	cancel(errors.New("test shutdown")) // request is mid-flight

	if statusCode := <-statusCodes; statusCode != 200 {
		t.Errorf("in-flight request status = %d, want 200", statusCode)
	}
	if err := <-runResult; err != nil {
		t.Errorf("Run returned %v, want nil", err)
	}
	if _, err := http.Get("http://" + address + "/"); err == nil {
		t.Error("server still accepts connections after shutdown")
	}
}

func TestRunReportsDrainTimeout(t *testing.T) {
	requestStarted := make(chan struct{})
	stuckHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(requestStarted)
		time.Sleep(2 * time.Second)
	})
	address := freeAddress(t)
	server := NewServer(address, stuckHandler, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() { runResult <- Run(ctx, server, 100*time.Millisecond, discardLogger()) }()

	go func() {
		for range 50 {
			if response, err := http.Get("http://" + address + "/"); err == nil {
				response.Body.Close()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	<-requestStarted
	cancel()
	if err := <-runResult; !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Run returned %v, want context.DeadlineExceeded", err)
	}
}

func TestRunCancelsRequestContextsOnShutdown(t *testing.T) {
	requestStarted := make(chan struct{})
	// Stands in for a readiness ping that waits on a hung dependency.
	waitingHandler := http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		close(requestStarted)
		select {
		case <-request.Context().Done():
			responseWriter.WriteHeader(http.StatusServiceUnavailable)
		case <-time.After(10 * time.Second):
			responseWriter.WriteHeader(http.StatusOK)
		}
	})

	address := freeAddress(t)
	server := NewServer(address, waitingHandler, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)
	go func() { runResult <- Run(ctx, server, 5*time.Second, discardLogger()) }()

	statusCodes := make(chan int, 1)
	go func() {
		for range 50 {
			response, err := http.Get("http://" + address + "/")
			if err == nil {
				response.Body.Close()
				statusCodes <- response.StatusCode
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		statusCodes <- 0
	}()

	<-requestStarted
	cancel()

	if statusCode := <-statusCodes; statusCode != http.StatusServiceUnavailable {
		t.Errorf("in-flight request status = %d, want 503", statusCode)
	}
	if err := <-runResult; err != nil {
		t.Errorf("Run returned %v, want nil", err)
	}
}
