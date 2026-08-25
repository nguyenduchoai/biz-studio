package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"bizstudio/internal/store"
)

func TestMergeSettingsPreservesMaskedSecrets(t *testing.T) {
	cur := store.Settings{GeminiAPIKey: "real-secret", Theme: "light"}
	req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(`{"geminiApiKey":"`+secretMask+`","theme":"dark"}`))
	req.Header.Set("Content-Type", "application/json")
	got, err := mergeSettings(cur, req)
	if err != nil {
		t.Fatal(err)
	}
	if got.GeminiAPIKey != "real-secret" || got.Theme != "dark" {
		t.Fatalf("merge masked = %#v", got)
	}
}

func TestMaskedSettingsDoesNotReturnSecret(t *testing.T) {
	got := maskedSettings(store.Settings{GeminiAPIKey: "secret", OpenAIKey: "other"})
	if got.GeminiAPIKey != secretMask || got.OpenAIKey != secretMask {
		t.Fatalf("secret chưa được mask: %#v", got)
	}
}
