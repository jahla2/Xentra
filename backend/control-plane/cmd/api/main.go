package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/clients"
	"github.com/jahla2/Xentra/backend/control-plane/internal/httpapi"
)
func main() {
	repo := application.NewMemoryEnvironmentRepository()
	runnerClient := clients.NewRunnerClient()
	aiClient := clients.NewAIHTTPClient(envOrDefault("XENTRA_AI_URL", "http://localhost:8000"))
	router := httpapi.NewRouter(application.NewEnvironmentService(repo, runnerClient), application.NewInvestigationService(repo, runnerClient, aiClient))
	addr := envOrDefault("XENTRA_CONTROL_ADDR", ":8080")
	log.Printf("xentra control plane listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
func envOrDefault(name, fallback string) string { if v := os.Getenv(name); v != "" { return v }; return fallback }
