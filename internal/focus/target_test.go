package focus

import (
	"os"
	"testing"
	"time"

	"sessions/internal/registry"
)

func TestValidProvider(t *testing.T) {
	for _, provider := range []string{ProviderCode, ProviderCursor, ProviderZed, ProviderCustom} {
		if !ValidProvider(provider) {
			t.Fatalf("%s should be valid", provider)
		}
	}
	if ValidProvider("tmux") {
		t.Fatal("tmux should not be a v1 provider")
	}
}

func TestStatusExpiredTTL(t *testing.T) {
	expired := time.Now().Add(-time.Minute)
	status := Status(&registry.FocusTarget{ExpiresAt: &expired}, time.Now())
	if !status.Stale || status.Reason == "" {
		t.Fatalf("expected stale expired target, got %+v", status)
	}
}

func TestStatusLivePID(t *testing.T) {
	status := Status(&registry.FocusTarget{PID: os.Getpid()}, time.Now())
	if status.Stale {
		t.Fatalf("current process should be fresh, got %+v", status)
	}
}
