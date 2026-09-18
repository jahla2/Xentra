package application

import (
	"context"
	"testing"
)

type fakeExecutor struct{ outputs map[string]string }

func (f fakeExecutor) Run(_ context.Context, name string, _ ...string) (string, error) {
	return f.outputs[name], nil
}

func TestDiscoverReportsCapabilitiesAndResources(t *testing.T) {
	svc := NewDiscoveryService(fakeExecutor{outputs: map[string]string{
		"hostname": "prod-01\n",
		"uname": "Linux\n",
		"nproc": "8\n",
		"free": "Mem: 16000 8000 4000 1000 4000 7000\n",
		"df": "/dev/sda1 100G 40G 60G 40% /\n",
		"docker": "api\tUp 2 minutes\nworker\tUp 3 minutes\n",
		"systemctl": "systemd 255\n",
		"git": "git version 2.46\n",
		"curl": "curl 8\n",
		"getent": "getent\n",
	}})
	result := svc.Discover(context.Background())
	if result.Hostname != "prod-01" || result.OS != "Linux" {
		t.Fatalf("unexpected host discovery: %#v", result)
	}
	if result.CPU != "8 cores" || result.Memory == "" || result.Disk == "" {
		t.Fatalf("resource discovery missing: %#v", result)
	}
	if len(result.Containers) != 2 {
		t.Fatalf("expected running containers, got %#v", result.Containers)
	}
	if len(result.Capabilities) < 5 {
		t.Fatalf("unexpected capabilities: %#v", result.Capabilities)
	}
}

func TestDiscoveryOutputIsBounded(t *testing.T) {
	value := ""
	for i := 0; i < 3000; i++ {
		value += "x"
	}
	got := boundedDiscoveryOutput(value)
	if len(got) > 2052 {
		t.Fatalf("discovery output was not bounded: %d", len(got))
	}
}
