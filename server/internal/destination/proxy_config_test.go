package destination

import (
	"context"
	"net"
	"testing"
)

func TestConfiguredEgressFailsClosedOnBadConfiguration(t *testing.T) {
	t.Setenv("WAZE_EGRESS_PROXY", "http://evil.example:3128")
	dial := configuredDial((&net.Dialer{}).DialContext)
	if _, err := dial(context.Background(), "tcp", "8.8.8.8:443"); err == nil {
		t.Fatal("invalid proxy configuration was accepted")
	}
}
