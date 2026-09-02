package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
)

// serverWithCertificateValidFor starts a TLS server presenting a certificate
// that expires after the given duration, and returns its host:port.
func serverWithCertificateValidFor(t *testing.T, remaining time.Duration) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "datalab.test"},
		Issuer:       pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now().Add(-60 * 24 * time.Hour),
		NotAfter:     time.Now().Add(remaining),
		DNSNames:     []string{"datalab.test", "127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{
		Certificate: [][]byte{der}, PrivateKey: key,
	}}}
	server.StartTLS()
	t.Cleanup(server.Close)

	return server.Listener.Addr().String()
}

func certificateAlertsFor(t *testing.T, remaining time.Duration) []healthAlert {
	t.Helper()
	handlers := Handlers{publicURL: "https://" + serverWithCertificateValidFor(t, remaining)}
	return handlers.certificateAlerts()
}

// Traefik renews at roughly thirty days remaining. A certificate with three
// weeks left has therefore already missed a renewal, and that miss is the
// interesting moment - not the outage a fortnight later.
func TestAMissedRenewalWindowWarnsWellBeforeExpiry(t *testing.T) {
	alerts := certificateAlertsFor(t, 20*24*time.Hour)
	if len(alerts) != 1 {
		t.Fatalf("want one alert, got %+v", alerts)
	}
	if alerts[0].Severity != healthWarning {
		t.Fatalf("severity = %q, want warning", alerts[0].Severity)
	}
	if alerts[0].Scope != health.ScopePlatform {
		t.Fatalf("a certificate is a platform condition, got scope %q", alerts[0].Scope)
	}
}

func TestExpiryWithinAWeekIsCritical(t *testing.T) {
	alerts := certificateAlertsFor(t, 3*24*time.Hour)
	if len(alerts) != 1 || alerts[0].Severity != healthCritical {
		t.Fatalf("want one critical alert, got %+v", alerts)
	}
}

func TestAnExpiredCertificateIsReportedAsExpired(t *testing.T) {
	alerts := certificateAlertsFor(t, -time.Hour)
	if len(alerts) != 1 || alerts[0].Severity != healthCritical {
		t.Fatalf("want one critical alert, got %+v", alerts)
	}
	if alerts[0].Summary != "the TLS certificate has expired" {
		t.Fatalf("summary = %q", alerts[0].Summary)
	}
}

func TestAHealthyCertificateIsSilent(t *testing.T) {
	if alerts := certificateAlertsFor(t, 60*24*time.Hour); len(alerts) != 0 {
		t.Fatalf("a certificate two months from expiry raised %+v", alerts)
	}
}

// Unreachable is not the same as expiring. On an installation where the
// backend cannot reach its own public name, this check has nothing to say and
// must not invent a certificate problem.
func TestAnUnreachableAddressRaisesNothing(t *testing.T) {
	handlers := Handlers{publicURL: "https://127.0.0.1:1"}
	if alerts := handlers.certificateAlerts(); len(alerts) != 0 {
		t.Fatalf("an unreachable address raised %+v", alerts)
	}
}

func TestNoPublicAddressMeansNoCheck(t *testing.T) {
	for _, configured := range []string{"", "   ", "http://plain.example"} {
		handlers := Handlers{publicURL: configured}
		if alerts := handlers.certificateAlerts(); len(alerts) != 0 {
			t.Fatalf("publicURL %q raised %+v", configured, alerts)
		}
	}
}

// A bare domain in a setting is not a mistake worth failing over.
func TestCertificateHostAcceptsWhatAnOperatorWouldType(t *testing.T) {
	for input, want := range map[string]string{
		"https://datalab.example.com":      "datalab.example.com:443",
		"https://datalab.example.com:8443": "datalab.example.com:8443",
		"datalab.example.com":              "datalab.example.com:443",
		"  https://datalab.example.com/  ": "datalab.example.com:443",
		"http://datalab.example.com":       "",
		"":                                 "",
	} {
		if got := certificateHost(input); got != want {
			t.Errorf("certificateHost(%q) = %q, want %q", input, got, want)
		}
	}
}
