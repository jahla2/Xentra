package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jahla2/Xentra/backend/control-plane/internal/httpapi"
)

func main() {
	address := envOrDefault("XENTRA_CONTROL_ADDR", ":8080")
	log.Printf("xentra control plane listening on %s", address)
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
