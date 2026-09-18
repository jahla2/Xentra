package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jahla2/Xentra/backend/runner/internal/application"
	"github.com/jahla2/Xentra/backend/runner/internal/control"
	"github.com/jahla2/Xentra/backend/runner/internal/httpapi"
	"github.com/jahla2/Xentra/backend/runner/internal/security"
)

func main() {
	executor := application.OSExecutor{}
	discovery := application.NewDiscoveryService(executor)
	tools := application.NewToolService(executor)

	if controlURL := strings.TrimSpace(os.Getenv("XENTRA_CONTROL_URL")); controlURL != "" {
		runOutbound(controlURL, discovery, tools)
		return
	}

	runLegacyInbound(discovery, tools)
}

func runOutbound(
	controlURL string,
	discovery *application.DiscoveryService,
	tools *application.ToolService,
) {
	config := control.Config{
		BaseURL: controlURL,
		RunnerID: strings.TrimSpace(os.Getenv("XENTRA_RUNNER_ID")),
		RunnerToken: strings.TrimSpace(os.Getenv("XENTRA_RUNNER_TOKEN")),
		InsecureDev: envBool("XENTRA_CONTROL_INSECURE_DEV"),
		CertFile: strings.TrimSpace(os.Getenv("XENTRA_CONTROL_CLIENT_CERT")),
		KeyFile: strings.TrimSpace(os.Getenv("XENTRA_CONTROL_CLIENT_KEY")),
		CAFile: strings.TrimSpace(os.Getenv("XENTRA_CONTROL_SERVER_CA")),
		PollEvery: pollInterval(),
	}
	client, err := control.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	agent := control.NewAgent(client, discovery, tools, config.PollEvery)
	if err := agent.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func runLegacyInbound(discovery *application.DiscoveryService, tools *application.ToolService) {
	addr := envOrDefault("XENTRA_RUNNER_ADDR", ":8090")
	router := httpapi.NewRouter(discovery, tools)
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if envBool("XENTRA_RUNNER_INSECURE_DEV") {
		log.Printf("xentra legacy inbound Runner listening on %s in explicit insecure development mode", addr)
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

	log.Printf("xentra legacy inbound Runner listening on %s with mutual TLS", addr)
	log.Fatal(server.Serve(tls.NewListener(listener, tlsConfig)))
}

func pollInterval() time.Duration {
	seconds, err := strconv.Atoi(envOrDefault("XENTRA_CONTROL_POLL_SECONDS", "2"))
	if err != nil || seconds < 1 || seconds > 60 {
		seconds = 2
	}
	return time.Duration(seconds) * time.Second
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
