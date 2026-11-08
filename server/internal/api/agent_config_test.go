package api

import "testing"

func TestAgentConfigRequiresIssuerAndAudience(t *testing.T) {
	t.Setenv("AUTH0_AGENT_ISSUER", "")
	t.Setenv("AUTH0_AGENT_AUDIENCE", "")
	if _, err := NewWithAgent(nil, "key", "client"); err == nil {
		t.Fatal("missing issuer and audience must fail closed")
	}
}

func TestAgentConfigUsesConfiguredValues(t *testing.T) {
	t.Setenv("AUTH0_AGENT_ISSUER", "https://tenant.example.com/")
	t.Setenv("AUTH0_AGENT_AUDIENCE", "https://api.example.com/agent")
	h, err := NewWithAgent(nil, "key", "client")
	if err != nil {
		t.Fatal(err)
	}
	a := h.(*server).agent
	if a.issuer != "https://tenant.example.com/" || a.audience != "https://api.example.com/agent" {
		t.Fatal("configured claims not used")
	}
}
