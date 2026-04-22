package k8s

import (
	"crypto/x509"
	"fmt"
)

func newCertPool(caPEM []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("failed to parse kubernetes CA cert")
	}
	return pool, nil
}
