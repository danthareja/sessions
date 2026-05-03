package attention

import (
	"testing"

	"sessions/internal/config"
	"sessions/internal/registry"
)

func TestPolicyDefaults(t *testing.T) {
	policy := New(config.Default().Attention.States)
	if !policy.Attention(registry.AgentNeedsInput) || !policy.Attention(registry.AgentFailed) {
		t.Fatal("needs-input and failed should be attention states")
	}
	for _, state := range []string{registry.AgentReady, registry.AgentRunning, registry.AgentIdle} {
		if policy.Attention(state) {
			t.Fatalf("%s should be quiet by default", state)
		}
	}
}

func TestPolicyUsesConfiguredStates(t *testing.T) {
	policy := New([]string{registry.AgentReady})
	if !policy.Attention(registry.AgentReady) {
		t.Fatal("ready should be configured as an attention state")
	}
	if policy.Attention(registry.AgentNeedsInput) {
		t.Fatal("needs-input should not be attention without configuration")
	}
}
