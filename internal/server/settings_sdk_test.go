package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"bizstudio/internal/store"
)

func TestSDKSettingsKeyMaskAndLimits(t *testing.T) {
	cur := store.Settings{AnthropicAPIKey: "fake-test-key", ClaudeBackend: "sdk"}
	if maskedSettings(cur).AnthropicAPIKey != secretMask {
		t.Fatal("SDK key is not masked")
	}
	req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(`{"anthropicApiKey":"`+secretMask+`","claudeSdkBudgetUsd":3}`))
	got, err := mergeSettings(cur, req)
	if err != nil || got.AnthropicAPIKey != cur.AnthropicAPIKey || got.ClaudeSDKBudgetUSD != 3 {
		t.Fatal("SDK masked roundtrip failed", err)
	}
	for _, body := range []string{`{"claudeBackend":"unknown"}`, `{"claudeBackend":"sdk","anthropicApiKey":""}`, `{"claudeSdkBudgetUsd":-1}`, `{"claudeSdkBudgetUsd":101}`} {
		r := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body))
		if _, err := mergeSettings(cur, r); err == nil {
			t.Fatalf("accepted invalid SDK settings: %s", body)
		}
	}
	r := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(`{"claudeBackend":"cli","anthropicApiKey":""}`))
	if _, err := mergeSettings(cur, r); err != nil {
		t.Fatal("cannot switch back to CLI and clear key", err)
	}
}
