package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Verification off is a decision, and it should look like one in the code that
// makes it. These check the three states an installation can be in.
func TestHarborClientVerifiesWhenNothingSaysOtherwise(t *testing.T) {
	client := Handlers{}.harborHTTPClient(time.Second)
	if client.Transport != nil {
		t.Fatal("with no CA and no skip flag the client must use the default, verifying transport")
	}
	if client.Timeout != time.Second {
		t.Errorf("timeout = %v, want 1s", client.Timeout)
	}
}

func TestHarborClientSkipsOnlyWhenAskedTo(t *testing.T) {
	client := Handlers{harborInsecureSkipVerify: true}.harborHTTPClient(time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("the skip flag must be the only thing that turns verification off")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Error("even when skipping verification, the platform must not negotiate an obsolete TLS version")
	}
}

// A pinned certificate beats the skip flag. An installation with both is one
// that added the CA and forgot the old flag, and the safe reading of that is
// the stricter one.
func TestAPinnedCertificateWinsOverTheSkipFlag(t *testing.T) {
	path := writeTestCertificate(t)
	resetHarborRoots()

	client := Handlers{harborCAFile: path, harborInsecureSkipVerify: true}.harborHTTPClient(time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		t.Fatal("a configured CA must produce a transport")
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("a configured CA must leave verification on, whatever the skip flag says")
	}
	if transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("the pinned certificate must be the root the client trusts")
	}
}

// A CA that cannot be read must not fall back to trusting everything. The call
// fails instead, which is the outcome that gets noticed and fixed.
func TestAnUnreadableCADoesNotBecomeNoVerification(t *testing.T) {
	resetHarborRoots()
	client := Handlers{harborCAFile: filepath.Join(t.TempDir(), "absent.pem"), harborInsecureSkipVerify: true}.harborHTTPClient(time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		t.Fatal("expected a transport")
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("a missing CA file must never turn into skipped verification")
	}
}

// The pool is memoised, which is right in production and wrong in a test that
// needs several configurations. Resetting the memo is the price of not adding
// an injection seam nobody would use outside tests.
func resetHarborRoots() {
	harborRootsOnce = sync.Once{}
	harborRoots = nil
	harborRootsErr = nil
}

func writeTestCertificate(t *testing.T) string {
	t.Helper()
	// Any syntactically valid certificate: what is under test is the wiring,
	// not the cryptography.
	pool := x509.NewCertPool()
	if pool == nil {
		t.Fatal("no certificate pool")
	}
	pem := `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----
`
	path := filepath.Join(t.TempDir(), "harbor-ca.pem")
	if err := os.WriteFile(path, []byte(pem), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
