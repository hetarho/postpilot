package rpcserver

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestListenFailureShutsDownEveryServer(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	failed := &http.Server{Addr: occupied.Addr().String()}
	other := &http.Server{Addr: "127.0.0.1:0"}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := Serve(ctx, failed, other); err == nil {
		t.Fatal("listen failure lost")
	}
	if err := other.ListenAndServe(); err != http.ErrServerClosed {
		t.Fatal("other listener survived failure", err)
	}
}
