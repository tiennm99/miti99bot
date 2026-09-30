package gold

import "testing"

func TestValidateEndpoint(t *testing.T) {
	if err := validateEndpoint("https://example.com/path"); err != nil {
		t.Fatalf("https should pass: %v", err)
	}
	if err := validateEndpoint("http://localhost:1234/path"); err != nil {
		t.Fatalf("localhost http should pass: %v", err)
	}
	if err := validateEndpoint("http://127.0.0.1:1234/path"); err != nil {
		t.Fatalf("loopback http should pass: %v", err)
	}
	if err := validateEndpoint("http://example.com/path"); err == nil {
		t.Fatal("remote http should fail")
	}
}
