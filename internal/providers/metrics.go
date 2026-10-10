package providers

import (
	"context"
	"net/http"
	"sync/atomic"
)

type requestCounterKey struct{}

func WithRequestCounter(ctx context.Context, counter *atomic.Uint64) context.Context {
	return context.WithValue(ctx, requestCounterKey{}, counter)
}

func doRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	if counter, ok := req.Context().Value(requestCounterKey{}).(*atomic.Uint64); ok {
		counter.Add(1)
	}
	return client.Do(req)
}
