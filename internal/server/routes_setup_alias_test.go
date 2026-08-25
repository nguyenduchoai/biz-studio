package server

import "testing"

func TestCanonicalSetupAliasMatchesRunningKey(t *testing.T) {
	if got := canonicalSetupID("yt-dlp"); got != "ytdlp" {
		t.Fatalf("alias yt-dlp = %q, muốn ytdlp", got)
	}
}
