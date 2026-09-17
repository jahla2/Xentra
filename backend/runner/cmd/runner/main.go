package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jahla2/Xentra/backend/runner/internal/application"
	"github.com/jahla2/Xentra/backend/runner/internal/httpapi"
)
func main() {
	addr := envOrDefault("XENTRA_RUNNER_ADDR", ":8090")
	executor := application.OSExecutor{}
	router := httpapi.NewRouter(application.NewDiscoveryService(executor), application.NewToolService(executor))
	log.Printf("xentra runner listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
func envOrDefault(name, fallback string) string { if v := os.Getenv(name); v != "" { return v }; return fallback }
