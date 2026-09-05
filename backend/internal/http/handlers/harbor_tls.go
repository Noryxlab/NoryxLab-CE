package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// How the platform trusts its registry.
//
// The backend talked to Harbor with certificate verification disabled, on both
// production platforms, because Harbor presents a certificate signed by nobody
// a public root store knows - which is the normal state of a registry on a
// private network. Skipping verification is the shortcut that makes it work,
// and it accepts any certificate at all: on a network where an attacker can
// answer for harbor.lan, the platform hands over its registry credentials.
//
// The honest fix is not a public certificate. It is telling the platform which
// certificate to expect: NORYX_HARBOR_CA_FILE points at Harbor's own, mounted
// from a secret, and verification is then on. A self-signed certificate that is
// pinned is stronger than a public one that is not checked.
var (
	harborRootsOnce sync.Once
	harborRoots     *x509.CertPool
	harborRootsErr  error
)

func harborCertificatePool(path string) (*x509.CertPool, error) {
	harborRootsOnce.Do(func() {
		pem, err := os.ReadFile(path)
		if err != nil {
			harborRootsErr = err
			return
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			harborRootsErr = &harborCAError{path: path}
			return
		}
		harborRoots = pool
	})
	return harborRoots, harborRootsErr
}

type harborCAError struct{ path string }

func (e *harborCAError) Error() string {
	return "no certificate found in " + e.path
}

// harborHTTPClient builds the client used to reach the registry.
//
// Order matters: a configured CA wins over skip-verify. An installation that
// sets both is one that added the CA and forgot to remove the old flag, and the
// safe reading of that is the stricter one - with a line in the log, because a
// setting that is silently ignored is its own kind of trap.
func (h Handlers) harborHTTPClient(timeout time.Duration) *http.Client {
	client := &http.Client{Timeout: timeout}

	if path := strings.TrimSpace(h.harborCAFile); path != "" {
		pool, err := harborCertificatePool(path)
		if err != nil {
			// Refusing to fall back to skip-verify: a misconfigured CA must
			// not quietly become no verification at all. The call fails, and
			// the message says why.
			log.Printf("harbor CA %s could not be read (%v); registry calls will fail until it is fixed", path, err)
			client.Transport = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12}}
			return client
		}
		if h.harborInsecureSkipVerify {
			log.Printf("both NORYX_HARBOR_CA_FILE and NORYX_HARBOR_INSECURE_SKIP_VERIFY are set; the CA wins and verification stays on")
		}
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}
		return client
	}

	if h.harborInsecureSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, //nolint:gosec // deliberate, and reported by the health check
		}
	}
	return client
}
