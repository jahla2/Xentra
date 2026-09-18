package application

import (
	"context"
	"fmt"
	"net/url"
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
	if _, err := s.exec.Run(ctx, "git", "--version"); err == nil { caps = append(caps, "git") }
	if _, err := s.exec.Run(ctx, "curl", "--version"); err == nil { caps = append(caps, "http") }
	if _, err := s.exec.Run(ctx, "getent", "--version"); err == nil { caps = append(caps, "dns") }
	return domain.Discovery{OS: runtime.GOOS, Hostname: strings.TrimSpace(host), Capabilities: caps}
}

type ToolService struct { exec Executor }
func NewToolService(exec Executor) *ToolService { return &ToolService{exec: exec} }

var safeToolTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]*$`)
var safeCommitPattern = regexp.MustCompile(`^[A-Fa-f0-9]{7,64}$`)

func (s *ToolService) Execute(ctx context.Context, tool string, args map[string]string) domain.ToolResult {
	var name string
	var commandArgs []string

	switch tool {
	case "system.info":
		name, commandArgs = "uname", []string{"-a"}
	case "system.disk":
		name, commandArgs = "df", []string{"-h"}
	case "system.cpu":
		name, commandArgs = "lscpu", nil
	case "system.memory":
		name, commandArgs = "free", []string{"-m"}
	case "docker.list":
		name, commandArgs = "docker", []string{"ps", "--format", "{{.Names}}\t{{.Status}}"}
	case "docker.logs":
		container := args["container"]
		if !safeToolTarget(container) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid container is required"} }
		name, commandArgs = "docker", []string{"logs", "--tail", "200", container}
	case "docker.inspect":
		container := args["container"]
		if !safeToolTarget(container) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid container is required"} }
		name, commandArgs = "docker", []string{"inspect", container}
	case "docker.stats":
		container := args["container"]
		if !safeToolTarget(container) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid container is required"} }
		name, commandArgs = "docker", []string{"stats", "--no-stream", "--format", "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}", container}
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
	case "system.journal":
		service := args["service"]
		if !safeToolTarget(service) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid service is required"} }
		name, commandArgs = "journalctl", []string{"-u", service, "-n", "200", "--no-pager"}
	case "git.status":
		repoPath, ok := safeRepoPath(args["path"])
		if !ok { return domain.ToolResult{Tool: tool, Success: false, Error: "valid repository path is required"} }
		name, commandArgs = "git", []string{"-C", repoPath, "status", "--short", "--branch"}
	case "git.log":
		repoPath, ok := safeRepoPath(args["path"])
		if !ok { return domain.ToolResult{Tool: tool, Success: false, Error: "valid repository path is required"} }
		name, commandArgs = "git", []string{"-C", repoPath, "log", "-n", "10", "--oneline", "--decorate"}
	case "git.diff":
		repoPath, ok := safeRepoPath(args["path"])
		if !ok { return domain.ToolResult{Tool: tool, Success: false, Error: "valid repository path is required"} }
		name, commandArgs = "git", []string{"-C", repoPath, "diff", "--stat"}
	case "git.show_commit":
		repoPath, ok := safeRepoPath(args["path"])
		if !ok { return domain.ToolResult{Tool: tool, Success: false, Error: "valid repository path is required"} }
		commit := strings.TrimSpace(args["commit"])
		if !safeCommitPattern.MatchString(commit) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid commit SHA is required"} }
		name, commandArgs = "git", []string{"-C", repoPath, "show", "--stat", "--oneline", "--decorate", "--no-renames", commit}
	case "http.health_check":
		targetURL := strings.TrimSpace(args["url"])
		if !safeHTTPURL(targetURL) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid http/https URL is required"} }
		name, commandArgs = "curl", []string{"--fail", "--silent", "--show-error", "--location", "--max-time", "8", "-o", "/dev/null", "-w", "%{http_code}", targetURL}
	case "dns.lookup":
		host := strings.TrimSpace(args["host"])
		if !safeToolTarget(host) { return domain.ToolResult{Tool: tool, Success: false, Error: "valid hostname is required"} }
		name, commandArgs = "getent", []string{"hosts", host}
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

func safeRepoPath(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "."
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", false
	}
	return value, true
}

func safeHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
