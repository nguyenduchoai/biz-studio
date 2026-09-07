package htmlvideo

import "testing"

func TestSceneFileURLsAcrossDesktopPlatforms(t *testing.T) {
	for _, tc := range []struct {
		name, path, want string
		windows          bool
	}{
		{"Windows drive and escaped filename", `C:\Users\An\Dữ liệu\scene #1%done.html`, "file:///C:/Users/An/D%E1%BB%AF%20li%E1%BB%87u/scene%20%231%25done.html", true},
		{"Windows forward slashes", "d:/video/scene.html", "file:///d:/video/scene.html", true},
		{"Windows network share", `\\NAS\video share\scene #1.html`, "file://NAS/video%20share/scene%20%231.html", true},
		{"Windows long drive path", `\\?\C:\video\scene.html`, "file:///C:/video/scene.html", true},
		{"Windows long network path", `\\?\UNC\NAS\video share\scene.html`, "file://NAS/video%20share/scene.html", true},
		{"macOS filename", "/tmp/Dữ liệu/scene #1%done.html", "file:///tmp/D%E1%BB%AF%20li%E1%BB%87u/scene%20%231%25done.html", false},
		{"Unix literal backslash", `/tmp/scene\name.html`, "file:///tmp/scene%5Cname.html", false},
		{"Unix question mark", "/tmp/scene?.html", "file:///tmp/scene%3F.html", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fileURLForPlatform(tc.path, tc.windows); got != tc.want {
				t.Fatalf("file URL for %q = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
