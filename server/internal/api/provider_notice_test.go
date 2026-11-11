package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderPrivacyNoticeDoesNotClaimPhotonAlways(t *testing.T) {
	a := New(nil, "synthetic")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(w.Body.String(), "a consulta é enviada ao operador Photon") {
		t.Fatal("notice incorrectly claims Photon is always used")
	}
}
