package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
)

// Certificate expiry.
//
// An expired certificate is a total outage, and it is the one outage that
// announces itself weeks in advance to anybody who looks. Nobody looks.
//
// Traefik renews from its ACME resolver at roughly thirty days remaining, so
// the platform is not being asked to renew anything here - it is being asked to
// notice when renewal has stopped working. That is why the warning sits well
// inside the renewal window: a certificate with three weeks left has already
// missed at least one renewal, and the interesting moment is that miss, not the
// expiry a fortnight later.
const (
	certificateWarningWindow  = 21 * 24 * time.Hour
	certificateCriticalWindow = 7 * 24 * time.Hour
	certificateDialTimeout    = 8 * time.Second
)

// certificateAlerts reports on the certificate served for the platform's own
// public address.
//
// It reads the certificate the way a browser does - a TLS handshake against
// the public name - rather than reading a file or asking Traefik. A file on
// disk can be renewed while the process still serves the old one, and that
// difference is the whole failure.
func (h Handlers) certificateAlerts() []healthAlert {
	host := certificateHost(h.publicURL)
	if host == "" {
		return nil
	}

	certificate, err := peerCertificate(host)
	if err != nil {
		// Unreachable is not the same as expiring, and must not be reported as
		// a certificate problem: on an installation where the backend cannot
		// reach its own public name - a split-horizon DNS, an egress rule -
		// this check simply has nothing to say.
		log.Printf("certificate check: cannot read the certificate for %s: %v", host, err)
		return nil
	}

	remaining := time.Until(certificate.NotAfter)
	notBefore := certificate.NotBefore
	switch {
	case remaining <= 0:
		return []healthAlert{{
			Scope: health.ScopePlatform, Severity: healthCritical, Source: "certificate",
			Summary: "the TLS certificate has expired",
			Detail:  certificateDetail(certificate, remaining),
			Since:   &certificate.NotAfter,
			Action:  "settings",
		}}
	case remaining <= certificateCriticalWindow:
		return []healthAlert{{
			Scope: health.ScopePlatform, Severity: healthCritical, Source: "certificate",
			Summary: "the TLS certificate expires in less than a week and has not renewed",
			Detail:  certificateDetail(certificate, remaining),
			Since:   &notBefore,
			Action:  "settings",
		}}
	case remaining <= certificateWarningWindow:
		return []healthAlert{{
			Scope: health.ScopePlatform, Severity: healthWarning, Source: "certificate",
			Summary: "the TLS certificate has missed its renewal window",
			Detail:  certificateDetail(certificate, remaining),
			Since:   &notBefore,
			Action:  "settings",
		}}
	}
	return nil
}

func certificateDetail(certificate *x509.Certificate, remaining time.Duration) string {
	days := int(remaining.Hours() / 24)
	detail := certificate.Subject.CommonName + " expires " +
		certificate.NotAfter.UTC().Format(time.RFC3339)
	if remaining > 0 {
		detail += " (" + strconv.Itoa(days) + " days)"
	}
	if issuer := strings.TrimSpace(certificate.Issuer.CommonName); issuer != "" {
		detail += ", issued by " + issuer
	}
	return detail
}

// certificateHost extracts host:port from the configured public URL, defaulting
// to 443. A URL without a scheme is accepted, because an operator writing a
// domain name into a setting is not making a mistake worth failing over.
func certificateHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	if parsed.Scheme != "https" {
		// Nothing to check: the platform is not published over TLS here.
		return ""
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	return net.JoinHostPort(parsed.Hostname(), port)
}

// peerCertificate completes a handshake and returns the leaf certificate.
//
// Verification is deliberately skipped: an expired or untrusted chain is
// exactly what this wants to report, and a verifying dial would fail before
// handing back the certificate that explains why.
func peerCertificate(hostPort string) (*x509.Certificate, error) {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: certificateDialTimeout}
	connection, err := tls.DialWithDialer(dialer, "tcp", hostPort, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true, //nolint:gosec // see the comment above
		MinVersion:         tls.VersionTLS12,
	})
	if err != nil {
		return nil, err
	}
	defer connection.Close()

	chain := connection.ConnectionState().PeerCertificates
	if len(chain) == 0 {
		return nil, errNoCertificate
	}
	return chain[0], nil
}

type certificateError string

func (e certificateError) Error() string { return string(e) }

const errNoCertificate certificateError = "the server presented no certificate"
