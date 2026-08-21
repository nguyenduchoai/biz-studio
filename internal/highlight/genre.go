package highlight

import "strings"

// Genre — thể loại nội dung của video nguồn.
//
// Vì sao cần: "đoạn đắt" không phải một tiêu chuẩn chung. Với video kiến thức,
// đoạn đắt là chỗ giải thích gọn một ý khó. Với vlog đời thường, đoạn đắt là
// chỗ có cảm xúc thật — mà đem thước đo kiến thức ra chấm thì nó rớt hết vì
// "không có thông tin". Một prompt duy nhất luôn mang gu của một thể loại, và
// âm thầm chấm sai mọi thể loại còn lại.
type Genre struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`

	// high — điều gì làm một đoạn đáng 9-10 điểm ở thể loại này.
	high string
	// low — điều gì kéo điểm xuống, ngoài các lỗi chung.
	low string
}

// Genres — danh mục thể loại. "auto" đứng đầu và là mặc định.
func Genres() []Genre {
	return []Genre{
		{
			ID: "auto", Name: "Tự cân bằng", Desc: "Chưa chắc video thuộc loại nào thì để mục này.",
			high: "câu gây tò mò, con số bất ngờ, tuyên bố mạnh, cao trào cảm xúc, hoặc một ý trọn vẹn đứng riêng vẫn hiểu",
			low:  "đoạn chỉ có giá trị khi đã nghe phần trước",
		},
		{
			ID: "kienthuc", Name: "Kiến thức · dạy học", Desc: "Giảng bài, hướng dẫn, giải thích khái niệm.",
			high: "giải thích gọn một ý khó, một cách làm cụ thể làm theo được ngay, một hiểu lầm phổ biến được đính chính, hoặc con số/ví dụ khiến người nghe à lên",
			low:  "đoạn chỉ dẫn nhập, nêu mục lục, hoặc nhắc lại điều vừa nói",
		},
		{
			ID: "quandiem", Name: "Quan điểm · bình luận", Desc: "Podcast tranh luận, bình luận thời sự, chia sẻ góc nhìn.",
			high: "quan điểm nói thẳng và dám nhận, chỗ phản bác lại số đông, câu chốt sắc gọn, hoặc lập luận đi tới kết luận rõ ràng",
			low:  "đoạn rào trước đón sau, nói nước đôi, hoặc kể lại tin tức mà chưa có nhận định",
		},
		{
			ID: "giaitri", Name: "Giải trí · hài · vlog", Desc: "Vlog đời thường, clip hài, phản ứng, chơi game.",
			high: "chỗ bật cười, tình huống bất ngờ, phản ứng thật, câu đùa có cú chốt, hoặc khoảnh khắc đáng yêu/ngượng ngùng",
			low:  "đoạn dẫn dắt dài mà chưa tới điểm buồn cười, hoặc cười vì chuyện chỉ người trong cuộc mới hiểu",
		},
		{
			ID: "review", Name: "Trải nghiệm · review", Desc: "Đánh giá sản phẩm, du lịch, ăn uống, mở hộp.",
			high: "nhận xét dứt khoát nên mua hay không, chỗ chỉ ra nhược điểm thật, so sánh cụ thể, hoặc phản ứng đầu tiên khi vừa trải nghiệm",
			low:  "đoạn đọc thông số mà không kèm đánh giá, hoặc khen chung chung không có căn cứ",
		},
		{
			ID: "kinhdoanh", Name: "Kinh doanh · bán hàng", Desc: "Chia sẻ kinh doanh, bán hàng, marketing, khởi nghiệp.",
			high: "con số doanh thu/chi phí thật, một sai lầm phải trả giá, cách làm áp dụng được ngay, hoặc điều ngược với lời khuyên thường nghe",
			low:  "đoạn nói đạo lý chung chung, hoặc quảng cáo lộ liễu không có nội dung",
		},
		{
			ID: "phongvan", Name: "Phỏng vấn · talkshow", Desc: "Trò chuyện với khách mời, hỏi đáp.",
			high: "câu trả lời thật lòng ngoài kịch bản, chuyện chưa kể bao giờ, chỗ khách mời ngập ngừng rồi nói thẳng, hoặc câu hỏi xoáy và phản ứng của khách",
			low:  "đoạn giới thiệu khách mời, chào hỏi, hoặc câu trả lời khuôn mẫu",
		},
	}
}

// FindGenre tra thể loại theo ID; không có thì trả về "auto".
func FindGenre(id string) Genre {
	want := strings.ToLower(strings.TrimSpace(id))
	gs := Genres()
	for _, g := range gs {
		if g.ID == want {
			return g
		}
	}
	return gs[0]
}
