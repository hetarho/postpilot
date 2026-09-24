package rpcserver

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Serve shuts down every listener on cancellation or any listen failure.
func Serve(ctx context.Context, servers ...*http.Server) error {
	failures := make(chan error, len(servers))
	for _, server := range servers {
		go func() {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				failures <- err
			}
		}()
	}
	var result error
	select {
	case <-ctx.Done():
	case result = <-failures:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, server := range servers {
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			result = errors.Join(result, err)
		}
	}
	return result
}
