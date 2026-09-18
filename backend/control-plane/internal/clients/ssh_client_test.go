package clients
import"testing"
func TestSafeDockerNameRejectsShellSyntax(t *testing.T){cases:=map[string]bool{"api-prod":true,"worker_1":true,"api;rm -rf /":false,"$(touch /tmp/pwn)":false};for value,want:=range cases{if got:=safeDockerName(value);got!=want{t.Fatalf("safeDockerName(%q)=%v want %v",value,got,want)}}}
