package clients

import (
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

func TestValidateSSMEnvironment(t *testing.T) {
	valid := domain.Environment{AWSRegion: "ap-southeast-2", AWSInstanceID: "i-0123456789abcdef0"}
	if err := validateSSMEnvironment(valid); err != nil {
		t.Fatalf("expected valid SSM target: %v", err)
	}

	invalid := []domain.Environment{
		{AWSRegion: "bad region", AWSInstanceID: "i-0123456789abcdef0"},
		{AWSRegion: "ap-southeast-2", AWSInstanceID: "instance-123"},
	}
	for _, env := range invalid {
		if err := validateSSMEnvironment(env); err == nil {
			t.Fatalf("expected invalid SSM target to fail: %#v", env)
		}
	}
}

func TestParseSSMSections(t *testing.T) {
	output := "__XENTRA_OS__\nLinux\n__XENTRA_HOSTNAME__\nip-10-0-0-25\n__XENTRA_CPU__\n4\n__XENTRA_CAPABILITIES__\ndocker\nsystemd\n__XENTRA_CONTAINERS__\napi\tUp 1 minute\n"
	sections := parseSSMSections(output)
	if sections["OS"] != "Linux" || sections["HOSTNAME"] != "ip-10-0-0-25" || sections["CPU"] != "4" {
		t.Fatalf("unexpected sections: %#v", sections)
	}
	if sections["CAPABILITIES"] != "docker\nsystemd" {
		t.Fatalf("unexpected capabilities section: %q", sections["CAPABILITIES"])
	}
}

func TestSSMReadPathSharesTypedAllowlist(t *testing.T) {
	command, err := sshReadToolCommand(domain.ToolRequest{
		Tool: "docker.logs",
		Arguments: map[string]string{"container": "api-prod"},
	})
	if err != nil || command == "" {
		t.Fatalf("expected typed docker logs command, command=%q err=%v", command, err)
	}
	if _, err := sshReadToolCommand(domain.ToolRequest{
		Tool: "docker.restart",
		Arguments: map[string]string{"container": "api-prod"},
	}); err == nil {
		t.Fatal("mutation must not be available through SSM read-tool path")
	}
}
