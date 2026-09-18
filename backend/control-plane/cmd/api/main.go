package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/clients"
	"github.com/jahla2/Xentra/backend/control-plane/internal/httpapi"
	"github.com/jahla2/Xentra/backend/control-plane/internal/persistence"
	"github.com/jahla2/Xentra/backend/control-plane/internal/security"
)

type repositoriesSet struct {
	auth                application.AuthRepository
	projects            application.ProjectRepository
	environments        application.EnvironmentRepository
	credentials         application.CredentialRepository
	integrations        application.RepositoryIntegrationRepository
	webhookDeliveries   application.WebhookDeliveryRepository
	incidents           application.IncidentRepository
	incidentMemory      application.IncidentMemoryRepository
	runnerControl       application.RunnerControlRepository
	actions             application.ActionRepository
	audit               application.AuditRepository
	db                  *sql.DB
}

type serverFailure struct {
	name string
	err  error
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stores := repositories(ctx)
	if stores.db != nil {
		defer stores.db.Close()
	}

	box, err := clients.NewAESSecretBox(masterKey(stores.db != nil))
	if err != nil {
		log.Fatal(err)
	}
	credentialService := application.NewCredentialService(stores.credentials, box)
	authService := application.NewAuthService(stores.auth, sessionTTL())

	runnerClient, err := clients.NewRunnerClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	sshClient := clients.NewSSHClient(credentialService)
	runnerControlService := application.NewRunnerControlService(
		stores.runnerControl,
		stores.environments,
		stores.projects,
		credentialService,
	)
	outboundRunnerClient := clients.NewOutboundRunnerClient(runnerControlService)
	connections := clients.NewConnectionClient(runnerClient, sshClient, outboundRunnerClient)
	aiClient := clients.NewAIHTTPClient(envOrDefault("XENTRA_AI_URL", "http://localhost:8000"))
	githubAuth, err := clients.NewGitHubAuthProvider(
		credentialService,
		os.Getenv("XENTRA_GITHUB_APP_ID"),
		githubPrivateKey(),
		envOrDefault("XENTRA_GITHUB_API_URL", "https://api.github.com"),
	)
	if err != nil {
		log.Fatal(err)
	}
	githubClient := clients.NewGitHubClient(githubAuth)

	projectService := application.NewProjectService(stores.projects)
	environmentService := application.NewEnvironmentService(stores.environments, stores.projects, connections, credentialService)
	memoryService := application.NewIncidentMemoryService(stores.incidentMemory, aiClient)
	investigationService := application.NewInvestigationService(stores.environments, connections, aiClient, memoryService)
	integrationService := application.NewIntegrationService(stores.integrations, credentialService, stores.environments, githubAuth)
	incidentService := application.NewIncidentService(stores.incidents, stores.integrations, investigationService, githubClient, memoryService)
	webhookService := application.NewGitHubWebhookService(stores.integrations, credentialService, stores.webhookDeliveries, incidentService)
	actionService := application.NewActionService(stores.actions, stores.audit, stores.environments, stores.incidents, connections)

	apiRouter := httpapi.NewRouter(
		environmentService,
		investigationService,
		httpapi.Services{
			Auth: authService, Projects: projectService, Integrations: integrationService, Webhooks: webhookService,
			Runners: runnerControlService, Incidents: incidentService, Actions: actionService,
		},
	)
	runnerRouter := httpapi.NewRunnerControlRouter(runnerControlService)

	apiAddr := envOrDefault("XENTRA_CONTROL_ADDR", ":8080")
	apiServer := newHTTPServer(apiAddr, apiRouter)

	runnerAddr := strings.TrimSpace(os.Getenv("XENTRA_RUNNER_CONTROL_ADDR"))
	var runnerServer *http.Server
	if runnerAddr != "" {
		runnerServer = newHTTPServer(runnerAddr, runnerRouter)
	}

	serverErr := make(chan serverFailure, 2)
	go func() {
		log.Printf("xentra control plane listening on %s", apiAddr)
		serverErr <- serverFailure{name: "api", err: apiServer.ListenAndServe()}
	}()
	if runnerServer != nil {
		go func() {
			serverErr <- serverFailure{name: "runner-control", err: serveRunnerControl(runnerServer)}
		}()
	}

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var unexpected *serverFailure
	select {
	case <-signalContext.Done():
	case failure := <-serverErr:
		if failure.err != nil && !errors.Is(failure.err, http.ErrServerClosed) {
			unexpected = &failure
		}
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := apiServer.Shutdown(shutdownContext); err != nil {
		log.Printf("control plane shutdown error: %v", err)
	}
	if runnerServer != nil {
		if err := runnerServer.Shutdown(shutdownContext); err != nil {
			log.Printf("runner control shutdown error: %v", err)
		}
	}

	if unexpected != nil {
		log.Fatalf("%s server failed: %v", unexpected.name, unexpected.err)
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func serveRunnerControl(server *http.Server) error {
	if envBool("XENTRA_RUNNER_CONTROL_INSECURE_DEV") {
		log.Printf("xentra runner control listening on %s in explicit insecure development mode", server.Addr)
		return server.ListenAndServe()
	}

	tlsConfig, err := security.ServerTLSConfig(
		strings.TrimSpace(os.Getenv("XENTRA_RUNNER_CONTROL_TLS_CERT")),
		strings.TrimSpace(os.Getenv("XENTRA_RUNNER_CONTROL_TLS_KEY")),
		strings.TrimSpace(os.Getenv("XENTRA_RUNNER_CONTROL_CLIENT_CA")),
	)
	if err != nil {
		return err
	}
	server.TLSConfig = tlsConfig

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	log.Printf("xentra runner control listening on %s with mutual TLS", server.Addr)
	return server.Serve(tls.NewListener(listener, tlsConfig))
}

func repositories(ctx context.Context) repositoriesSet {
	databaseURL := os.Getenv("XENTRA_DATABASE_URL")
	if databaseURL == "" {
		log.Print("XENTRA_DATABASE_URL not set; using in-memory repositories")
		return repositoriesSet{
			auth:              application.NewMemoryAuthRepository(),
			projects:          application.NewMemoryProjectRepository(),
			environments:      application.NewMemoryEnvironmentRepository(),
			credentials:       application.NewMemoryCredentialRepository(),
			integrations:      application.NewMemoryIntegrationRepository(),
			webhookDeliveries: application.NewMemoryWebhookDeliveryRepository(),
			incidents:         application.NewMemoryIncidentRepository(),
			incidentMemory:    application.NewMemoryIncidentMemoryRepository(),
			runnerControl:     application.NewMemoryRunnerControlRepository(),
			actions:           application.NewMemoryActionRepository(),
			audit:             application.NewMemoryAuditRepository(),
		}
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	if err := persistence.Migrate(ctx, db); err != nil {
		log.Fatal(err)
	}

	return repositoriesSet{
		auth:              persistence.NewPostgresAuthRepository(db),
		projects:          persistence.NewPostgresProjectRepository(db),
		environments:      persistence.NewPostgresEnvironmentRepository(db),
		credentials:       persistence.NewPostgresCredentialRepository(db),
		integrations:      persistence.NewPostgresIntegrationRepository(db),
		webhookDeliveries: persistence.NewPostgresWebhookDeliveryRepository(db),
		incidents:         persistence.NewPostgresIncidentRepository(db),
		incidentMemory:    persistence.NewPostgresIncidentMemoryRepository(db),
		runnerControl:     persistence.NewPostgresRunnerControlRepository(db),
		actions:           persistence.NewPostgresActionRepository(db),
		audit:             persistence.NewPostgresAuditRepository(db),
		db:                db,
	}
}

func sessionTTL() time.Duration {
	hours, err := strconv.Atoi(envOrDefault("XENTRA_SESSION_TTL_HOURS", "24"))
	if err != nil || hours < 1 || hours > 720 {
		log.Fatal("XENTRA_SESSION_TTL_HOURS must be between 1 and 720")
	}
	return time.Duration(hours) * time.Hour
}

func masterKey(persistent bool) []byte {
	encoded := os.Getenv("XENTRA_MASTER_KEY")
	if encoded != "" {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(key) != 32 {
			log.Fatal("XENTRA_MASTER_KEY must be base64 for exactly 32 bytes")
		}
		return key
	}
	if persistent {
		log.Fatal("XENTRA_MASTER_KEY is required when PostgreSQL persistence is enabled")
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		log.Fatal(err)
	}
	log.Print("XENTRA_MASTER_KEY not set; generated ephemeral development key")
	return key
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

func githubPrivateKey() string {
	if value := os.Getenv("XENTRA_GITHUB_APP_PRIVATE_KEY"); value != "" {
		return value
	}
	if path := os.Getenv("XENTRA_GITHUB_APP_PRIVATE_KEY_FILE"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read XENTRA_GITHUB_APP_PRIVATE_KEY_FILE: %v", err)
		}
		return string(raw)
	}
	return ""
}
