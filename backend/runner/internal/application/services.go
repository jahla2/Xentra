package application

import (
	"context"
	"fmt"
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
func (s *ToolService) Execute(ctx context.Context, tool string, args map[string]string) domain.ToolResult {
	var name string
	var commandArgs []string
	switch tool {
	case "system.info": name, commandArgs = "uname", []string{"-a"}
	case "system.disk": name, commandArgs = "df", []string{"-h"}
	case "docker.list": name, commandArgs = "docker", []string{"ps", "--format", "{{.Names}}\t{{.Status}}"}
	case "docker.logs":
		container := args["container"]
		if container == "" { return domain.ToolResult{Tool: tool, Success: false, Error: "container is required"} }
		name, commandArgs = "docker", []string{"logs", "--tail", "200", container}
	default:
		return domain.ToolResult{Tool: tool, Success: false, Error: "tool is not allowlisted"}
	}
	output, err := s.exec.Run(ctx, name, commandArgs...)
	if err != nil { return domain.ToolResult{Tool: tool, Success: false, Error: fmt.Sprintf("%v", err)} }
	return domain.ToolResult{Tool: tool, Success: true, Output: output}
}
