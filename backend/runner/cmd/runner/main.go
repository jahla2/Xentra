package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jahla2/Xentra/backend/runner/internal/application"
	"github.com/jahla2/Xentra/backend/runner/internal/httpapi"
	"github.com/jahla2/Xentra/backend/runner/internal/security"
)

func main() {
	addr := envOrDefault("XENTRA_RUNNER_ADDR", ":8090")
	executor := application.OSExecutor{}
	router := httpapi.NewRouter(application.NewDiscoveryService(executor), application.NewToolService(executor))
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if envBool("XENTRA_RUNNER_INSECURE_DEV") {
		log.Printf("xentra runner listening on %s in explicit insecure development mode", addr)
		log.Fatal(server.ListenAndServe())
	}

	tlsConfig, err := security.ServerTLSConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	server.TLSConfig = tlsConfig

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Printf("xentra runner listening on %s with mutual TLS", addr)
	log.Fatal(server.Serve(tls.NewListener(listener, tlsConfig)))
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
