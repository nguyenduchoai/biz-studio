package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDesktopFirstRunWizardEnabledForSupportedPlatforms(t *testing.T) {
	for _, tc := range []struct {
		platform string
		enabled  bool
	}{{"windows", true}, {"darwin", true}, {"linux", false}} {
		t.Run(tc.platform, func(t *testing.T) {
			s := newTestServer(t)
			s.Platform = tc.platform
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:6868/api/instance", nil)
			markLocalControlRequest(req)
			s.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("instance API: %d %s", rec.Code, rec.Body.String())
			}
			var response struct {
				Platform      string `json:"platform"`
				WizardEnabled bool   `json:"wizardEnabled"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Platform != tc.platform || response.WizardEnabled != tc.enabled {
				t.Fatalf("platform %s cannot enter expected first-run flow: %+v", tc.platform, response)
			}
		})
	}
}
