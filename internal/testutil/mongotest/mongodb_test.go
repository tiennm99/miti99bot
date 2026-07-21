package mongotest

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestManagerURIUsesExternalOverride(t *testing.T) {
	t.Setenv("MONGODB_TEST_URL", " mongodb://example.test:27017 ")

	var manager Manager
	if got := manager.URI(t); got != "mongodb://example.test:27017" {
		t.Fatalf("URI = %q, want external override", got)
	}
}

func TestManagerURISkipsWithWarningWhenDockerUnavailable(t *testing.T) {
	t.Setenv("MONGODB_TEST_URL", "")
	var warning bytes.Buffer
	manager := Manager{
		healthCheck: func() error { return errors.New("daemon unavailable") },
		warningOut:  &warning,
	}

	continued := false
	t.Run("without Docker", func(t *testing.T) {
		manager.URI(t)
		continued = true
	})
	if continued {
		t.Fatal("URI continued after Docker health failure; want test skip")
	}
	if got := warning.String(); !strings.Contains(got, "WARNING: Docker is not available; MongoDB integration tests skipped") {
		t.Fatalf("warning = %q", got)
	}
}
