package security

import "testing"

func TestServerTLSConfigRequiresCompleteMaterial(t *testing.T) {
	if _, err := ServerTLSConfig("", "", ""); err == nil {
		t.Fatal("expected missing TLS material to fail")
	}
}
