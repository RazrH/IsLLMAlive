package providers

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"isllmalive/internal/config"
	"isllmalive/internal/status"
)

func TestRoundHTTPSVerifiesCertificatesAndSupportsTLS(t *testing.T) {
	for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		t.Run(fmt.Sprintf("TLS-%x", version), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.TLS.Version != version {
					t.Errorf("TLS version = %x, want %x", r.TLS.Version, version)
				}
				fmt.Fprint(w, `{"status":{"indicator":"none"}}`)
			}))
			server.TLS = &tls.Config{MinVersion: version, MaxVersion: version}
			server.StartTLS()
			t.Cleanup(server.Close)
			m := config.MonitorConfig{Type: "statuspage", Name: "TLS", Endpoint: server.URL}
			untrusted := NewEngine()
			t.Cleanup(untrusted.Close)
			got := untrusted.NewRound(t.Context(), []config.MonitorConfig{m}).Fetch(t.Context(), m)
			if got.Status != status.Unknown || got.Err == nil || requests.Load() != 0 {
				t.Fatalf("untrusted certificate accepted: status=%s err=%v requests=%d", got.Status, got.Err, requests.Load())
			}
			if _, ok := errors.AsType[x509.UnknownAuthorityError](got.Err); !ok {
				t.Fatalf("expected certificate trust error, got %v", got.Err)
			}
			trusted := NewEngine()
			trusted.standard = server.Client()
			t.Cleanup(trusted.Close)
			got = trusted.NewRound(t.Context(), []config.MonitorConfig{m}).Fetch(t.Context(), m)
			if got.Status != status.Normal || got.Err != nil || requests.Load() != 1 {
				t.Fatalf("trusted HTTPS failed: status=%s err=%v requests=%d", got.Status, got.Err, requests.Load())
			}
		})
	}
}
