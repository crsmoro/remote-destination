package destination

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestRelayModeDoesNotResolveGoogleLocally(t *testing.T) {
	t.Setenv("WAZE_EGRESS_PROXY", "egress:3128")
	r := &fakeResolver{}
	var target string
	dial := func(_ context.Context, _, address string) (net.Conn, error) {
		target = address
		return nil, errors.New("stop")
	}
	_, _ = expand("https://maps.app.goo.gl/test", r, dial)
	if target != "maps.app.goo.gl:443" {
		t.Fatalf("relay target = %q, want Google hostname", target)
	}
}
