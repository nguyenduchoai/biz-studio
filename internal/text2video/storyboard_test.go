package text2video

import (
	"strings"
	"testing"

	"bizstudio/internal/store"
)

// Tên nhân vật TUYỆT ĐỐI không được lọt vào prompt sinh ảnh: model thiên vị rất
// nặng với tên riêng, đặt tên trùng nhân vật nổi tiếng là nó vẽ nhân vật nó đã
// học chứ không vẽ nhân vật của người dùng. Bài kiểm tra này khoá cứng luật đó.
func TestCharacterClauseKhongLoTen(t *testing.T) {
	st := openTestStore(t)
	cases := []struct{ name, look string }{
		{"Elsa", "a young woman with long blonde hair in a blue dress"},
		{"Naruto", "a teenage boy in an orange jacket"},
		{"Chị Lan", "phụ nữ 30 tuổi, áo dài trắng"},
		{"Sơn Tùng", "young man with dyed hair"},
	}
	for _, c := range cases {
		ch := store.Character{Name: c.name, Look: c.look}
		st.SaveCharacter(&ch)
		got := CharacterClause(st, []string{ch.ID})
		if strings.Contains(got, c.name) {
			t.Errorf("tên %q lọt vào prompt: %q", c.name, got)
		}
		if !strings.Contains(got, c.look) {
			t.Errorf("mất mô tả ngoại hình của %q: %q", c.name, got)
		}
	}
}

// Nhân vật chỉ có tên, chưa tả ngoại hình: phải BỎ QUA hẳn. Đưa mỗi cái tên vào
// prompt còn hại hơn không đưa gì.
func TestCharacterClauseBoQuaNhanVatChuaTa(t *testing.T) {
	st := openTestStore(t)
	trong := store.Character{Name: "Elsa"}
	st.SaveCharacter(&trong)
	if got := CharacterClause(st, []string{trong.ID}); got != "" {
		t.Errorf("nhân vật chưa tả ngoại hình phải cho ra chuỗi rỗng, nhận %q", got)
	}

	// Có tả thì vẫn dùng bình thường, và nhân vật rỗng không làm hỏng cả mệnh đề.
	co := store.Character{Name: "Minh", Look: "a tall man in a grey coat"}
	st.SaveCharacter(&co)
	got := CharacterClause(st, []string{trong.ID, co.ID})
	if !strings.Contains(got, "a tall man in a grey coat") {
		t.Errorf("nhân vật có tả phải còn trong prompt: %q", got)
	}
	if strings.Contains(got, "Elsa") || strings.Contains(got, "Minh") {
		t.Errorf("vẫn còn tên trong prompt: %q", got)
	}
}

// Nhiều nhân vật: các mô tả nối nhau bằng dấu chấm, không trùng lặp id.
func TestCharacterClauseNhieuNhanVat(t *testing.T) {
	st := openTestStore(t)
	a := store.Character{Name: "A", Look: "an old fisherman with a hunched back"}
	b := store.Character{Name: "B", Look: "a young woman carrying a paper umbrella"}
	st.SaveCharacter(&a)
	st.SaveCharacter(&b)

	got := CharacterClause(st, []string{a.ID, b.ID, a.ID}) // a lặp lại
	if n := strings.Count(got, "an old fisherman"); n != 1 {
		t.Errorf("nhân vật lặp id phải chỉ xuất hiện 1 lần, nhận %d: %q", n, got)
	}
	if !strings.Contains(got, "a young woman carrying") {
		t.Errorf("thiếu nhân vật thứ hai: %q", got)
	}
	if !strings.HasPrefix(got, characterLead) {
		t.Errorf("thiếu mở đầu %q: %q", characterLead, got)
	}
}

// Không có nhân vật nào / store nil → chuỗi rỗng, không panic.
func TestCharacterClauseRong(t *testing.T) {
	if got := CharacterClause(nil, []string{"x"}); got != "" {
		t.Errorf("store nil phải cho rỗng, nhận %q", got)
	}
	st := openTestStore(t)
	if got := CharacterClause(st, nil); got != "" {
		t.Errorf("không có id phải cho rỗng, nhận %q", got)
	}
	if got := CharacterClause(st, []string{"khong-co-that"}); got != "" {
		t.Errorf("id không tồn tại phải cho rỗng, nhận %q", got)
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("mở store thử nghiệm: %v", err)
	}
	return st
}
