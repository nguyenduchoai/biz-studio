package store

import "strings"

const defaultClaudeModel = "claude-opus-5"

// normalizeClaudeModel upgrades retired Opus selections while preserving any
// other model the user chose explicitly. The short `opus` alias still resolved
// to Opus 4.8 in Claude Code 2.1.205, so Biz Studio uses the current full name.
func normalizeClaudeModel(model string) string {
	model = strings.TrimSpace(model)
	switch strings.ToLower(model) {
	case "", "opus", "opus[1m]", "claude-opus-4-8", "claude-opus-4-8[1m]":
		return defaultClaudeModel
	default:
		return model
	}
}
