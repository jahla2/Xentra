package application

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/jahla2/Xentra/backend/runner/internal/domain"
)

type Executor interface { Run(context.Context, string, ...string) (string, error) }

type DiscoveryService struct { exec Executor }
func NewDiscoveryService(exec Executor) *DiscoveryService { return &DiscoveryService{exec: exec} }
func (s *DiscoveryService) Discover(ctx context.Context) domain.Discovery {
	host, _ := s.exec.Run(ctx, "hostname")
	caps := []string{}
	if _, err := s.exec.Run(ctx, "docker", "--version"); err == nil { caps = append(caps, "docker") }
	if _, err := s.exec.Run(ctx, "systemctl", "--version"); err == nil { caps = append(caps, "systemd") }
	return domain.Discovery{OS: runtime.GOOS, Hostname: strings.TrimSpace(host), Capabilities: caps}
}

type ToolService struct { exec Executor }
func NewToolService(exec Executor) *ToolService { return &ToolService{exec: exec} }

var safeToolTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]*$`)

func (s *ToolService) Execute(ctx context.Context, tool string, args map[string]string) domain.ToolResult {
	var name string
	var commandArgs []string

	switch tool {
	case "system.info":
		name, commandArgs = "uname", []string{"-a"}
	case "system.disk":
		name, commandArgs = "df", []string{"-h"}
	case "docker.list":
		name, commandArgs = "docker", []string{"ps", "--format", "{{.Names}}\t{{.Status}}"}
	case "docker.logs":
		container := args["container"]
		if !safeToolTarget(container) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid container is required"} }
		name, commandArgs = "docker", []string{"logs", "--tail", "200", container}
	case "docker.restart":
		container := args["container"]
		if !safeToolTarget(container) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid container is required"} }
		name, commandArgs = "docker", []string{"restart", container}
	case "docker.status":
		container := args["container"]
		if !safeToolTarget(container) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid container is required"} }
		name, commandArgs = "docker", []string{"inspect", "-f", "{{.State.Running}}", container}
	case "system.service_restart":
		service := args["service"]
		if !safeToolTarget(service) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid service is required"} }
		name, commandArgs = "systemctl", []string{"restart", service}
	case "system.service_status":
		service := args["service"]
		if !safeToolTarget(service) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid service is required"} }
		name, commandArgs = "systemctl", []string{"is-active", service}
	default:
		return domain.ToolResult{Tool: tool, Success: false, Error: "tool is not allowlisted"}
	}

	output, err := s.exec.Run(ctx, name, commandArgs...)
	if err != nil {
		return domain.ToolResult{Tool: tool, Success: false, Error: fmt.Sprintf("%v", err), Output: output}
	}
	return domain.ToolResult{Tool: tool, Success: true, Output: output}
}

func safeToolTarget(value string) bool {
	return value != "" && safeToolTargetPattern.MatchString(value)
}
