package agent

import (
	"context"
	"testing"
)

func TestP0CapabilityRegistryContainsReadOnlyCapabilities(t *testing.T) {
	registry := NewP0CapabilityRegistry()
	capability, ok := registry.Get("feed.query_recent_items")
	if !ok {
		t.Fatal("feed.query_recent_items was not registered")
	}
	if capability.Mutates {
		t.Fatal("feed.query_recent_items should be read-only")
	}
	if capability.Mode != CapabilityModeCore {
		t.Fatalf("mode = %q, want core", capability.Mode)
	}
}

func TestP0CapabilityRegistryContainsWebCapabilities(t *testing.T) {
	registry := NewP0CapabilityRegistry()
	for _, key := range []string{"web.search", "web.fetch_page", "web.extract_page", "repo.search", "repo.inspect_remote"} {
		capability, ok := registry.Get(key)
		if !ok {
			t.Fatalf("%s was not registered", key)
		}
		if !capability.ExternalAccess {
			t.Fatalf("%s should be marked as external access", key)
		}
		if capability.Mutates {
			t.Fatalf("%s should be read-only", key)
		}
	}
}

func TestPolicyEngineAllowsReadOnlyAndPromptsMutatingCapability(t *testing.T) {
	engine := NewPolicyEngine()
	readOnly := Capability{Key: "feed.query_recent_items", Risk: CapabilityRiskLow}
	if decision := engine.Decide(context.Background(), PolicyInput{Capability: readOnly, UserID: 1}); decision.Decision != PolicyDecisionAllow {
		t.Fatalf("read-only decision = %q, want allow", decision.Decision)
	}

	mutating := Capability{Key: "source.subscribe", Risk: CapabilityRiskMedium, Mutates: true}
	if decision := engine.Decide(context.Background(), PolicyInput{Capability: mutating, UserID: 1}); decision.Decision != PolicyDecisionPrompt {
		t.Fatalf("mutating decision = %q, want prompt", decision.Decision)
	}
}

func TestPolicyEngineForbidsMissingUser(t *testing.T) {
	engine := NewPolicyEngine()
	capability := Capability{Key: "feed.query_recent_items", Risk: CapabilityRiskLow}
	if decision := engine.Decide(context.Background(), PolicyInput{Capability: capability}); decision.Decision != PolicyDecisionForbidden {
		t.Fatalf("decision = %q, want forbidden", decision.Decision)
	}
}
