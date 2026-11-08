package main

import "testing"

func TestConfiguredIssuerHost(t *testing.T) {
	t.Setenv("AUTH0_AGENT_ISSUER", "https://tenant.example.com/")
	hosts, err := configuredHosts()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, h := range hosts {
		if h == "tenant.example.com" {
			found = true
		}
	}
	if !found {
		t.Fatal("configured issuer missing from allowlist")
	}
}
