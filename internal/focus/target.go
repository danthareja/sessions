package focus

import (
	"fmt"
	"time"

	"sessions/internal/registry"
)

const (
	ProviderCode   = "code"
	ProviderCursor = "cursor"
	ProviderZed    = "zed"
	ProviderCustom = "custom"
)

type TargetStatus struct {
	Stale  bool
	Reason string
}

func ValidProvider(provider string) bool {
	switch provider {
	case ProviderCode, ProviderCursor, ProviderZed, ProviderCustom:
		return true
	default:
		return false
	}
}

func EditorProvider(provider string) bool {
	switch provider {
	case ProviderCode, ProviderCursor, ProviderZed:
		return true
	default:
		return false
	}
}

func Status(target *registry.FocusTarget, now time.Time) TargetStatus {
	if target == nil {
		return TargetStatus{}
	}
	if target.ExpiresAt != nil && !target.ExpiresAt.After(now) {
		return TargetStatus{Stale: true, Reason: "ttl expired"}
	}
	if target.PID > 0 && !pidAlive(target.PID) {
		return TargetStatus{Stale: true, Reason: fmt.Sprintf("pid %d is not running", target.PID)}
	}
	return TargetStatus{}
}
