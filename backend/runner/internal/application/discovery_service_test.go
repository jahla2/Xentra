package application

import (
	"context"
	"testing"
)

type fakeExecutor struct{ outputs map[string]string }
func (f fakeExecutor) Run(_ context.Context, name string, _ ...string) (string, error) { return f.outputs[name], nil }
func TestDiscoverReportsCapabilities(t *testing.T) {
	svc := NewDiscoveryService(fakeExecutor{outputs: map[string]string{"hostname":"prod-01\n","docker":"Docker version 27\n","systemctl":"systemd 255\n"}})
	result := svc.Discover(context.Background())
	if result.Hostname != "prod-01" || len(result.Capabilities) < 2 { t.Fatalf("unexpected discovery: %#v", result) }
}
