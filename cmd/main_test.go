package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeDrainsActiveRequestOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	started := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	shutdown := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "completed before MCP sessions close")
	})}
	server.RegisterOnShutdown(func() { close(shutdown) })
	done := make(chan error, 1)
	go func() { done <- serve(ctx, server, listener) }()
	t.Cleanup(func() { _ = server.Close() })

	type response struct {
		body string
		err  error
	}
	responseCh := make(chan response, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			responseCh <- response{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		responseCh <- response{body: string(body), err: err}
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not enter handler")
	}
	cancel()
	select {
	case <-shutdown:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not initiate shutdown")
	}
	select {
	case err := <-done:
		t.Fatalf("server returned before in-flight request completed: %v", err)
	default:
	}
	close(release)
	select {
	case result := <-responseCh:
		if result.err != nil || result.body != "completed before MCP sessions close" {
			t.Fatalf("in-flight response = %q, error = %v", result.body, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight response was not drained")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not return after draining request")
	}
}
