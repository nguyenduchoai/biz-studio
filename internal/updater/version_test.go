package updater

import "testing"

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"2.14.0-rc.2", "2.14.0-rc.3", -1},
		{"2.14.0-rc.3", "2.14.0", -1},
		{"2.14.0", "2.14.0-rc.3", 1},
		{"2.15.0", "2.14.9", 1},
		{"v2.14.0", "2.14.0", 0},
	}
	for _, tt := range tests {
		a, err := parseVersion(tt.a)
		if err != nil {
			t.Fatal(err)
		}
		b, err := parseVersion(tt.b)
		if err != nil {
			t.Fatal(err)
		}
		if got := compareVersion(a, b); got != tt.want {
			t.Errorf("compareVersion(%q, %q) = %d, muốn %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := assetName("windows", "amd64"); got != "BizStudio-windows-amd64.zip" {
		t.Fatal(got)
	}
	if got := assetName("darwin", "arm64"); got != "BizStudio-macos-arm64.tar.gz" {
		t.Fatal(got)
	}
	if got := assetName("linux", "arm64"); got != "BizStudio-linux-arm64.tar.gz" {
		t.Fatal(got)
	}
	if got := assetName("windows", "arm64"); got != "" {
		t.Fatalf("nền tảng chưa đóng gói phải bị chặn, got %q", got)
	}
}
