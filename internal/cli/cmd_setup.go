package cli

import (
	"context"
	"os"
	"strings"

	"bizstudio/internal/setup"
	"bizstudio/internal/util"
)

// runSetup cài hoặc cập nhật một công cụ ngoài.
//
//	bizstudio setup                 → liệt kê công cụ cài được
//	bizstudio setup yt-dlp --update → cập nhật (chữa lỗi 403 khi tải)
//	bizstudio setup whisper         → cài
//
// Có mặt ở CLI vì hai lý do: máy chủ không có màn hình vẫn cài được, và agent
// gặp lỗi KindDependency thì có đúng một câu lệnh để tự gỡ, thay vì dừng lại
// báo người dùng đi cài tay.
func runSetup(args []string) Result {
	f := fs("setup")
	update := f.Bool("update", false, "cập nhật thay vì cài mới")
	dry := f.Bool("dry-run", false, "chỉ in lệnh sẽ chạy, không chạy")
	data := f.String("data", util.DefaultDataDir(), "thư mục data của studio")
	if err := parse(f, args); err != nil {
		return Fail("setup", Usage("%s", err))
	}

	if f.NArg() == 0 {
		return Result{OK: true, Command: "setup", Outputs: map[string]any{"tools": toolList()}}
	}

	id := f.Arg(0)
	tool, ok := setup.Find(id)
	if !ok {
		return Fail("setup", Usage("không có công cụ %q — xem danh sách: bizstudio setup", id))
	}
	action := "install"
	if *update {
		action = "update"
	}

	tmp, err := os.MkdirTemp("", "bizstudio-setup-*")
	if err != nil {
		return Fail("setup", Failed("tạo thư mục tạm: %v", err))
	}
	defer os.RemoveAll(tmp)

	plan, err := setup.BuildPlan(tool, action, *data, tmp)
	if err != nil {
		// Thiếu brew/winget hay cần sudo không phải lỗi chạy — là thiếu điều
		// kiện. Agent đọc kind="dependency" thì biết phải xử lý cái đó trước.
		return Fail("setup", Dependency("%s", err))
	}

	res := Result{OK: true, Command: "setup", Outputs: map[string]any{
		"tool": tool.ID, "action": action, "cmds": plan.Cmds,
	}}
	if *dry {
		res.Outputs["dryRun"] = true
		return res
	}

	Logf("→ %s", strings.Join(plan.Cmds, " && "))
	if err := setup.Run(context.Background(), plan, func(line string) { Logf("%s", line) }); err != nil {
		return Fail("setup", Failed("%s %s thất bại: %v — tải thủ công: %s",
			action, tool.Label, err, tool.Manual))
	}
	return res
}

func toolList() []any {
	out := []any{}
	for _, t := range setup.Tools() {
		out = append(out, map[string]any{
			"id": t.ID, "label": t.Label, "desc": t.Desc, "manual": t.Manual,
		})
	}
	return out
}
