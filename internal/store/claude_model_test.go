package store

import "testing"

func TestNormalizeClaudeModel(t *testing.T) {
	tests := map[string]string{
		"":                        defaultClaudeModel,
		"opus":                    defaultClaudeModel,
		"opus[1m]":                defaultClaudeModel,
		"claude-opus-4-8":         defaultClaudeModel,
		"claude-opus-4-8[1m]":     defaultClaudeModel,
		" claude-opus-5 ":         defaultClaudeModel,
		"claude-sonnet-5":         "claude-sonnet-5",
		"custom-provider/model-x": "custom-provider/model-x",
	}
	for input, want := range tests {
		if got := normalizeClaudeModel(input); got != want {
			t.Errorf("normalizeClaudeModel(%q) = %q, muốn %q", input, got, want)
		}
	}
}

func TestSaveSettingsMigratesRetiredClaudeModel(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.Settings()
	cfg.ClaudeModel = "claude-opus-4-8[1m]"
	st.SaveSettings(cfg)
	if got := st.Settings().ClaudeModel; got != defaultClaudeModel {
		t.Fatalf("ClaudeModel = %q, muốn %q", got, defaultClaudeModel)
	}
}
