// Package cli — dòng lệnh cho Biz Studio, dùng được cho script và cho AI agent.
//
// Bốn quy ước, mỗi cái sinh ra từ một cách hỏng cụ thể khi máy đọc kết quả của
// máy:
//
//  1. Dòng CUỐI trên stdout luôn là MỘT dòng JSON. Log tiến độ đi hết sang
//     stderr. Bắt agent đọc log dạng chữ là bắt nó đoán, mà log thì đổi luôn.
//  2. Mỗi lệnh ghi manifest.json vào thư mục làm việc. Lệnh sau chỉ cần trỏ
//     đúng thư mục là dùng lại được kết quả lệnh trước, không phải chép tay
//     từng đường dẫn qua lại.
//  3. --dry-run kiểm tham số và dựng manifest mà KHÔNG chạy gì tốn kém. Agent
//     thử được câu lệnh trước khi đốt một tiếng render hay một lượt gọi AI.
//  4. Lỗi có PHÂN LOẠI (usage / dependency / retryable / failed). "Thất bại"
//     chung chung thì agent chỉ biết thử lại mù; biết loại thì nó biết nên sửa
//     tham số, cài thêm công cụ, hay thử lại.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Kind — phân loại lỗi để bên gọi biết phải làm gì tiếp.
type Kind string

const (
	KindUsage      Kind = "usage"      // sai tham số → sửa câu lệnh
	KindDependency Kind = "dependency" // thiếu ffmpeg/ffprobe/yt-dlp → cài thêm
	KindRetryable  Kind = "retryable"  // mạng, quá tải, hết hạn → thử lại
	KindFailed     Kind = "failed"     // chạy rồi nhưng hỏng → đọc message
)

// Err — lỗi có phân loại.
type Err struct {
	Kind Kind
	Msg  string
}

func (e *Err) Error() string { return e.Msg }

func Usage(format string, a ...any) error {
	return &Err{KindUsage, fmt.Sprintf(format, a...)}
}
func Dependency(format string, a ...any) error {
	return &Err{KindDependency, fmt.Sprintf(format, a...)}
}
func Failed(format string, a ...any) error {
	return &Err{KindFailed, fmt.Sprintf(format, a...)}
}

// KindOf đọc phân loại của một lỗi bất kỳ; lỗi thường → "failed".
func KindOf(err error) Kind {
	if e, ok := err.(*Err); ok {
		return e.Kind
	}
	return KindFailed
}

// Result — thân của dòng JSON cuối cùng trên stdout.
type Result struct {
	OK      bool           `json:"ok"`
	Command string         `json:"command"`
	Workdir string         `json:"workdir,omitempty"`
	Outputs map[string]any `json:"outputs,omitempty"`
	Stats   map[string]any `json:"stats,omitempty"`
	DryRun  bool           `json:"dryRun,omitempty"`
	Error   *errBody       `json:"error,omitempty"`
}

type errBody struct {
	Kind    Kind   `json:"kind"`
	Message string `json:"message"`
}

// manifestName — tên file trạng thái trong thư mục làm việc.
const manifestName = "bizstudio_manifest.json"

// Manifest — kết quả tích luỹ của các lệnh đã chạy trong cùng thư mục.
type Manifest struct {
	Version int                      `json:"version"`
	Stages  map[string]ManifestStage `json:"stages"`
	Outputs map[string]string        `json:"outputs"` // tên logic → đường dẫn
}

// ManifestStage — một lệnh đã chạy xong.
type ManifestStage struct {
	Command string            `json:"command"`
	At      string            `json:"at"`
	Outputs map[string]string `json:"outputs,omitempty"`
	Stats   map[string]any    `json:"stats,omitempty"`
	DryRun  bool              `json:"dryRun,omitempty"`
}

// LoadManifest đọc manifest trong thư mục làm việc; chưa có thì trả bản rỗng
// chứ KHÔNG lỗi — lệnh đầu tiên bao giờ cũng chạy trên thư mục trống.
func LoadManifest(workdir string) *Manifest {
	m := &Manifest{Version: 1, Stages: map[string]ManifestStage{}, Outputs: map[string]string{}}
	raw, err := os.ReadFile(filepath.Join(workdir, manifestName))
	if err != nil {
		return m
	}
	var got Manifest
	if json.Unmarshal(raw, &got) != nil {
		return m // manifest hỏng thì bỏ qua, đừng chặn người dùng chạy tiếp
	}
	if got.Stages == nil {
		got.Stages = map[string]ManifestStage{}
	}
	if got.Outputs == nil {
		got.Outputs = map[string]string{}
	}
	got.Version = 1
	return &got
}

// Save ghi manifest, đè phần của lệnh vừa chạy và gộp outputs vào bảng chung.
func (m *Manifest) Save(workdir, command string, st ManifestStage) error {
	if m.Stages == nil {
		m.Stages = map[string]ManifestStage{}
	}
	if m.Outputs == nil {
		m.Outputs = map[string]string{}
	}
	m.Stages[command] = st
	for k, v := range st.Outputs {
		m.Outputs[k] = v
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workdir, manifestName), append(raw, '\n'), 0o644)
}

// Get lấy một đường dẫn đã ghi trong manifest (rỗng nếu chưa có).
func (m *Manifest) Get(name string) string { return m.Outputs[name] }

// ---------- in kết quả ----------

// Logf in tiến độ ra STDERR. Stdout dành riêng cho một dòng JSON cuối cùng —
// lẫn log vào stdout là agent đọc JSON hỏng ngay.
func Logf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// Emit in dòng JSON kết quả rồi trả mã thoát.
func Emit(res Result) int {
	raw, err := json.Marshal(res)
	if err != nil {
		fmt.Fprintf(os.Stdout, `{"ok":false,"error":{"kind":"failed","message":%q}}`+"\n", err.Error())
		return 1
	}
	fmt.Fprintln(os.Stdout, string(raw))
	if res.OK {
		return 0
	}
	return exitCode(res.Error)
}

// exitCode — mã thoát theo loại lỗi, để script dùng được mà không phải đọc JSON.
func exitCode(e *errBody) int {
	if e == nil {
		return 1
	}
	switch e.Kind {
	case KindUsage:
		return 2
	case KindDependency:
		return 3
	case KindRetryable:
		return 4
	default:
		return 1
	}
}

// Fail dựng Result cho một lỗi.
func Fail(command string, err error) Result {
	return Result{
		OK: false, Command: command,
		Error: &errBody{Kind: KindOf(err), Message: err.Error()},
	}
}

// Now — mốc thời gian cho manifest, tách ra để test ghim được.
var Now = func() string { return time.Now().Format(time.RFC3339) }

// ---------- trợ giúp chung ----------

// Commands — bảng lệnh, dùng cho cả điều phối lẫn in trợ giúp.
type Command struct {
	Name  string
	Short string
	Run   func(args []string) Result
}

// Help in danh sách lệnh ra stderr.
func Help(cmds []Command) {
	Logf("Biz Studio — dòng lệnh cho script và AI agent\n")
	Logf("Dùng:  bizstudio <lệnh> [tuỳ chọn]")
	Logf("       bizstudio -port 6868 -data data     (không có lệnh = chạy giao diện web)\n")
	Logf("Lệnh:")
	names := make([]string, 0, len(cmds))
	byName := map[string]Command{}
	for _, c := range cmds {
		names = append(names, c.Name)
		byName[c.Name] = c
	}
	sort.Strings(names)
	for _, n := range names {
		Logf("  %-12s %s", n, byName[n].Short)
	}
	Logf("\nMọi lệnh in MỘT dòng JSON ra stdout khi xong; tiến độ đi ra stderr.")
	Logf("Thêm --dry-run để kiểm tham số mà không chạy gì tốn kém.")
	Logf("Mã thoát: 0 xong · 2 sai tham số · 3 thiếu công cụ · 4 nên thử lại · 1 hỏng khác.")
}

// IsServeMode — không có lệnh, hoặc đối số đầu bắt đầu bằng "-" thì chạy web.
//
// Bắt buộc phải giữ: bản .app trên macOS gọi thẳng `bizstudio -port 6868 -data …`,
// đổi cách điều phối mà quên ca này là app trên máy người dùng không mở nổi.
func IsServeMode(args []string) bool {
	if len(args) == 0 {
		return true
	}
	return strings.HasPrefix(args[0], "-")
}
