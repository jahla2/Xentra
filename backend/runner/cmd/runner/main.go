package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jahla2/Xentra/backend/runner/internal/httpapi"
)

func main() {
	address := envOrDefault("XENTRA_RUNNER_ADDR", ":8090")
	log.Printf("xentra runner listening on %s", address)
	if err := http.ListenAndServe(address, httpapi.NewRouter()); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
