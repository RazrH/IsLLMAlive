package providers

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const maxResponseBytes int64 = 8 << 20

var ErrResponseTooLarge = errors.New("decompressed response exceeds 8 MiB")

func readResponse(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}
func configureTransport(t *http.Transport) {
	t.IdleConnTimeout = 90 * time.Second
	t.MaxIdleConns = 16
	t.MaxIdleConnsPerHost = 2
	t.MaxConnsPerHost = 4
	t.ResponseHeaderTimeout = 3 * time.Second
	t.TLSHandshakeTimeout = 3 * time.Second
}
func newStandardClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	configureTransport(transport)
	return &http.Client{Transport: transport, Timeout: 3 * time.Second}
}
func (e *Engine) standardClient() *http.Client {
	e.httpMu.Lock()
	defer e.httpMu.Unlock()
	if e.standard == nil {
		e.standard = newStandardClient()
	}
	return e.standard
}
func (e *Engine) utlsClient() *http.Client {
	e.httpMu.Lock()
	defer e.httpMu.Unlock()
	if e.utls == nil {
		e.utls = newDeepSeekUTLSHTTPClient(3 * time.Second)
	}
	return e.utls
}

// CloseIdleConnections is also used when configuration retires sources.
// Shared transports close all idle connections; active requests are cancelled by the round.
func (e *Engine) CloseIdleConnections() {
	e.httpMu.Lock()
	defer e.httpMu.Unlock()
	if e.standard != nil {
		e.standard.CloseIdleConnections()
	}
	if e.utls != nil {
		e.utls.CloseIdleConnections()
	}
}
func (e *Engine) Close()           { e.CloseIdleConnections(); e.keys.invalidate() }
func responseError(code int) error { return fmt.Errorf("HTTP %d from status source", code) }
