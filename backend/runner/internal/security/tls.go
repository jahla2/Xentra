package security

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

func ServerTLSConfigFromEnv() (*tls.Config, error) {
	certFile := os.Getenv("XENTRA_RUNNER_TLS_CERT")
	keyFile := os.Getenv("XENTRA_RUNNER_TLS_KEY")
	clientCAFile := os.Getenv("XENTRA_RUNNER_CLIENT_CA")
	if certFile == "" || keyFile == "" || clientCAFile == "" {
		return nil, errors.New("XENTRA_RUNNER_TLS_CERT, XENTRA_RUNNER_TLS_KEY and XENTRA_RUNNER_CLIENT_CA are required")
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load runner server certificate: %w", err)
	}
	caPEM, err := os.ReadFile(clientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read runner client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("runner client CA contains no valid certificates")
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}, nil
}
