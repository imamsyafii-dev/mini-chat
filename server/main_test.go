package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/imamsyafii-dev/mini-chat/server/internal/httpapi"
)

func TestServeStopsOnContextCancellation(
	t *testing.T,
) {
	listener, err := net.Listen(
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf(
			"net.Listen() error = %v",
			err,
		)
	}

	server := &http.Server{
		Handler: httpapi.NewHandler(),
	}

	ctx, cancel :=
		context.WithCancel(
			context.Background(),
		)
	defer cancel()

	result := make(chan error, 1)

	go func() {
		result <- serve(
			ctx,
			listener,
			server,
			time.Second,
		)
	}()

	client := &http.Client{
		Timeout: 500 * time.Millisecond,
	}
	healthURL := "http://" +
		listener.Addr().String() +
		"/health"

	deadline := time.Now().Add(
		2 * time.Second,
	)

	for {
		response, requestErr :=
			client.Get(healthURL)
		if requestErr == nil {
			response.Body.Close()

			if response.StatusCode !=
				http.StatusOK {
				t.Fatalf(
					"health status = %d, want %d",
					response.StatusCode,
					http.StatusOK,
				)
			}

			break
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"server did not become ready: %v",
				requestErr,
			)
		}

		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf(
				"serve() error = %v, want nil",
				err,
			)
		}

	case <-time.After(2 * time.Second):
		t.Fatal(
			"serve() did not stop after context cancellation",
		)
	}
}
