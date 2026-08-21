package highlight

import (
	"context"
	"strings"
	"testing"
)

func scoredPool(n int) []Candidate {
	cs := candidates(n)
	for i := range cs {
		cs[i].Score = 8
	}
	return cs
}

// Trong một hợp tuyển, các đoạn phải xếp theo THỜI GIAN gốc dù AI liệt kê lộn
// xộn. Ghép theo thứ tự AI đưa thì câu chuyện nhảy cóc tới lui — cùng lý do đã
// buộc Pick phải sắp lại.
func TestClusterOrdersClipsByTime(t *testing.T) {
	fakeLLM(t, func(string) (string, error) {
		return `[{"tieu_de":"Chuyện khởi nghiệp","vi":"cùng chủ đề","doan":[9,2,5,0]}]`, nil
	})
	cols, err := Cluster(context.Background(), nil, scoredPool(20), 6, 4, 120, "auto")
	if err != nil {
		t.Fatalf("Cluster: %v", err)
	}
	if len(cols) != 1 {
		t.Fatalf("muốn 1 hợp tuyển, có %d", len(cols))
	}
	for i := 1; i < len(cols[0].Clips); i++ {
		if cols[0].Clips[i].Start < cols[0].Clips[i-1].Start {
			t.Fatalf("đoạn không theo thứ tự thời gian: %.0fs sau %.0fs",
				cols[0].Clips[i].Start, cols[0].Clips[i-1].Start)
		}
	}
}

// AI hay bịa số thứ tự không có trong danh sách. Bỏ qua chứ không được sập, và
// không được đưa vào một đoạn không tồn tại.
func TestClusterIgnoresInventedIndexes(t *testing.T) {
	fakeLLM(t, func(string) (string, error) {
		return `[{"tieu_de":"Nhóm A","doan":[0,1,2,999,1234]}]`, nil
	})
	cols, err := Cluster(context.Background(), nil, scoredPool(10), 6, 4, 120, "auto")
	if err != nil {
		t.Fatalf("Cluster: %v", err)
	}
	if len(cols[0].Clips) != 3 {
		t.Fatalf("giữ %d đoạn, muốn 3 (bỏ hai số bịa)", len(cols[0].Clips))
	}
}

// Một đoạn không được nằm ở hai hợp tuyển: dựng ra hai video có chung một khúc
// là người xem thấy lặp, và nội dung của nhóm sau bị loãng.
func TestClusterNoClipInTwoCollections(t *testing.T) {
	fakeLLM(t, func(string) (string, error) {
		return `[{"tieu_de":"A","doan":[0,1,2,3]},{"tieu_de":"B","doan":[2,3,4,5]}]`, nil
	})
	cols, err := Cluster(context.Background(), nil, scoredPool(10), 6, 4, 120, "auto")
	if err != nil {
		t.Fatalf("Cluster: %v", err)
	}
	seen := map[int]string{}
	for _, c := range cols {
		for _, cl := range c.Clips {
			if prev, dup := seen[cl.Index]; dup {
				t.Errorf("đoạn %d nằm ở cả %q lẫn %q", cl.Index, prev, c.Title)
			}
			seen[cl.Index] = c.Title
		}
	}
}

// Nhóm quá ít đoạn thì bỏ hẳn — gắn một cái tên kêu cho hai mẩu rời không làm
// nó thành hợp tuyển.
func TestClusterDropsTooSmallGroups(t *testing.T) {
	fakeLLM(t, func(string) (string, error) {
		return `[{"tieu_de":"Đủ","doan":[0,1,2,3]},{"tieu_de":"Hụt","doan":[4,5]}]`, nil
	})
	cols, err := Cluster(context.Background(), nil, scoredPool(10), 6, 4, 120, "auto")
	if err != nil {
		t.Fatalf("Cluster: %v", err)
	}
	if len(cols) != 1 || cols[0].Title != "Đủ" {
		t.Fatalf("muốn giữ đúng nhóm \"Đủ\", nhận %d nhóm", len(cols))
	}
}

// Bỏ nhóm hụt thì các đoạn của nó phải được TRẢ LẠI cho nhóm sau dùng, không
// được coi như đã tiêu.
func TestClusterReleasesClipsFromDroppedGroup(t *testing.T) {
	fakeLLM(t, func(string) (string, error) {
		return `[{"tieu_de":"Hụt","doan":[0,1]},{"tieu_de":"Sau","doan":[0,1,2,3]}]`, nil
	})
	cols, err := Cluster(context.Background(), nil, scoredPool(10), 6, 4, 120, "auto")
	if err != nil {
		t.Fatalf("Cluster: %v", err)
	}
	if len(cols) != 1 || len(cols[0].Clips) != 4 {
		t.Fatalf("nhóm \"Sau\" phải nhận đủ 4 đoạn, nhận %d — đoạn của nhóm bị bỏ chưa được trả lại",
			len(cols[0].Clips))
	}
}

// Trần thời lượng mỗi hợp tuyển phải được tôn trọng, nếu không video vượt trần
// nền tảng và bị xếp sang loại khác.
func TestClusterRespectsPerCollectionLimit(t *testing.T) {
	fakeLLM(t, func(string) (string, error) {
		return `[{"tieu_de":"A","doan":[0,1,2,3,4,5,6,7,8,9]}]`, nil
	})
	// Mỗi đoạn 8 giây; trần 30 giây → tối đa 3 đoạn.
	cols, err := Cluster(context.Background(), nil, scoredPool(20), 6, 4, 30, "auto")
	if err != nil {
		t.Fatalf("Cluster: %v", err)
	}
	if cols[0].Sec > 30 {
		t.Errorf("hợp tuyển dài %.0fs, vượt trần 30s", cols[0].Sec)
	}
}

func TestClusterErrorsWhenNothingGroups(t *testing.T) {
	fakeLLM(t, func(string) (string, error) { return `[{"tieu_de":"A","doan":[0,1]}]`, nil })
	_, err := Cluster(context.Background(), nil, scoredPool(10), 6, 4, 120, "auto")
	if err == nil {
		t.Fatal("không nhóm nào đủ đoạn mà vẫn báo thành công")
	}
	if !strings.Contains(err.Error(), "Rút clip ngắn") {
		t.Errorf("lỗi nên chỉ người dùng sang cách phù hợp hơn, đang là: %v", err)
	}
}

func TestClusterErrorsWhenTooFewHighScoring(t *testing.T) {
	cs := candidates(10) // điểm 0 hết
	if _, err := Cluster(context.Background(), nil, cs, 6, 4, 120, "auto"); err == nil {
		t.Fatal("không đoạn nào đạt điểm mà vẫn báo thành công")
	}
}

func TestParseClustersAcceptsEnglishKeys(t *testing.T) {
	got := parseClusters("```json\n" + `[{"title":"A","reason":"r","indexes":["3","7"]}]` + "\n```")
	if len(got) != 1 || got[0].Title != "A" || len(got[0].Indexes) != 2 || got[0].Indexes[0] != 3 {
		t.Fatalf("không đọc được khoá tiếng Anh / số dạng chuỗi: %+v", got)
	}
}
