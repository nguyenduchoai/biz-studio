// Package vtemplate — bộ khuôn làm video theo lĩnh vực.
//
// Một khuôn KHÔNG chỉ là câu prompt. Người mới mở phần mềm ra không biết chọn
// tỉ lệ nào, giọng nào, nhạc tone gì, mở đầu bao lâu — nên mỗi khuôn gói sẵn cả
// bộ: hướng viết kịch bản, phong cách hình, khung hình, nền tảng đích, kiểu
// giọng, tone nhạc và nhịp ba đoạn. Bấm một cái là có đủ khung để bắt đầu, đổi
// gì cũng được.
package vtemplate

import "strings"

// Template — một khuôn làm video.
type Template struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Icon     string `json:"icon"`
	Category string `json:"category"`
	Desc     string `json:"desc"`

	// Script — hướng dẫn viết kịch bản, ghép vào prompt khi sinh kịch bản.
	Script string `json:"script"`
	// Style — gợi ý bộ Style Kit (khớp theo tên, không khớp thì bỏ qua).
	Style string `json:"style"`

	Aspect   string `json:"aspect"`   // 9:16 | 16:9 | 1:1
	Platform string `json:"platform"` // id preset nền tảng
	Seconds  int    `json:"seconds"`  // thời lượng nhắm tới

	VoiceGender string `json:"voiceGender"` // Nam | Nữ | "" (tuỳ)
	VoicePace   string `json:"voicePace"`   // chậm | vừa | nhanh
	MusicMood   string `json:"musicMood"`   // id tone nhạc

	// Beats — nhịp ba đoạn: mở đầu giữ chân, thân, chốt.
	Hook string `json:"hook"`
	Body string `json:"body"`
	CTA  string `json:"cta"`
}

// Danh mục.
const (
	CatQuangCao  = "Quảng cáo & bán hàng"
	CatReview    = "Review & đánh giá"
	CatGiaoDuc   = "Kiến thức & giáo dục"
	CatGiaiTri   = "Giải trí"
	CatDoiSong   = "Đời sống"
	CatKeChuyen  = "Kể chuyện"
	CatDoanhNghi = "Doanh nghiệp"
)

var all = []Template{
	// ---------- Quảng cáo & bán hàng ----------
	{
		ID: "quang-cao-san-pham", Name: "Quảng cáo sản phẩm", Icon: "🛍", Category: CatQuangCao,
		Desc:   "TVC ngắn cho một sản phẩm: nêu vấn đề, đưa giải pháp, chốt mua.",
		Script: "Viết kịch bản quảng cáo sản phẩm dọc 9:16. Mở bằng NỖI ĐAU cụ thể của người mua, không nói tên sản phẩm ở câu đầu. Giữa video mới đưa sản phẩm ra như lời giải. Mỗi cảnh một ý, câu ngắn, có con số hoặc chi tiết cụ thể thay vì tính từ chung chung.",
		Style:  "Điện ảnh chân thực", Aspect: "9:16", Platform: "tiktok", Seconds: 30,
		VoicePace: "nhanh", MusicMood: "hao-hung",
		Hook: "Nêu đúng nỗi khó chịu người xem đang gặp, trong 3 giây đầu.",
		Body: "Sản phẩm giải quyết ra sao — cho thấy, đừng kể.",
		CTA:  "Một hành động duy nhất, rõ ràng.",
	},
	{
		ID: "review-san-pham", Name: "Review sản phẩm", Icon: "⭐", Category: CatReview,
		Desc:   "Đánh giá thật, có khen có chê, giọng người dùng thật.",
		Script: "Viết kịch bản review sản phẩm giọng người dùng thật, KHÔNG giọng quảng cáo. Bắt buộc có ít nhất một điểm CHÊ cụ thể — review chỉ khen thì không ai tin. Kết bằng khuyến nghị rõ: hợp với ai, không hợp với ai.",
		Style:  "Điện ảnh chân thực", Aspect: "9:16", Platform: "tiktok", Seconds: 45,
		VoicePace: "vừa", MusicMood: "nhe-nhang",
		Hook: "Câu kết luận đưa lên đầu: đáng mua hay không.",
		Body: "Ba điểm được, một điểm mất, kèm dẫn chứng.",
		CTA:  "Hợp với ai — nói thẳng.",
	},
	{
		ID: "so-sanh-hai-san-pham", Name: "So sánh hai lựa chọn", Icon: "⚖️", Category: CatReview,
		Desc:   "Đặt hai sản phẩm cạnh nhau, chốt nên chọn cái nào.",
		Script: "Viết kịch bản so sánh hai lựa chọn. Mỗi cảnh so MỘT tiêu chí, nói rõ bên nào thắng tiêu chí đó và vì sao. Không hoà cả làng — cuối cùng phải chốt nên chọn cái nào cho từng nhóm người dùng.",
		Style:  "Editorial 2D hiện đại", Aspect: "9:16", Platform: "tiktok", Seconds: 45,
		VoicePace: "vừa", MusicMood: "cang-thang",
		Hook: "Nêu thẳng hai cái tên và câu hỏi chọn cái nào.",
		Body: "So từng tiêu chí, mỗi tiêu chí một cảnh.",
		CTA:  "Chốt: người thế này chọn A, người thế kia chọn B.",
	},

	// ---------- Kiến thức & giáo dục ----------
	{
		ID: "kien-thuc-nhanh", Name: "Kiến thức 60 giây", Icon: "🎓", Category: CatGiaoDuc,
		Desc:   "Giải thích một khái niệm khó bằng ngôn ngữ đời thường.",
		Script: "Giải thích một khái niệm trong 60 giây cho người chưa biết gì. Cấm dùng thuật ngữ mà không giải nghĩa ngay. Dùng một phép so sánh đời thường xuyên suốt video thay vì đổi ví dụ liên tục.",
		Style:  "Phẳng tối giản", Aspect: "9:16", Platform: "shorts", Seconds: 60,
		VoicePace: "vừa", MusicMood: "nhe-nhang",
		Hook: "Một câu hỏi ai cũng từng thắc mắc.",
		Body: "Một phép so sánh đời thường, giữ nguyên tới cuối.",
		CTA:  "Câu chốt đọng lại một ý duy nhất.",
	},
	{
		ID: "lich-su-ke-lai", Name: "Lịch sử kể lại", Icon: "🏛", Category: CatGiaoDuc,
		Desc:   "Một sự kiện lịch sử kể như chuyện phim, có mốc thời gian.",
		Script: "Kể một sự kiện lịch sử theo mạch thời gian, giọng kể chuyện chứ không giọng sách giáo khoa. Mỗi cảnh gắn một MỐC cụ thể (năm, địa danh, con số). Không phán xét, để sự kiện tự nói.",
		Style:  "Điện ảnh chân thực", Aspect: "9:16", Platform: "tiktok", Seconds: 60,
		VoiceGender: "Nam", VoicePace: "chậm", MusicMood: "hung-trang",
		Hook: "Một chi tiết lạ khiến người ta muốn nghe tiếp.",
		Body: "Mạch thời gian, mỗi cảnh một mốc.",
		CTA:  "Hệ quả còn thấy tới hôm nay.",
	},
	{
		ID: "so-lieu-bao-cao", Name: "Video số liệu", Icon: "📊", Category: CatGiaoDuc,
		Desc:   "Biến bảng số khô khan thành câu chuyện có nhịp.",
		Script: "Biến số liệu thành câu chuyện. Mỗi cảnh MỘT con số duy nhất, nói rõ nó lớn hay nhỏ so với cái gì — số trần trụi không có mốc so sánh thì vô nghĩa. Kết bằng điều số liệu đang cảnh báo hoặc hứa hẹn.",
		Style:  "Editorial 2D hiện đại", Aspect: "9:16", Platform: "tiktok", Seconds: 45,
		VoicePace: "vừa", MusicMood: "cang-thang",
		Hook: "Con số gây sốc nhất đưa lên đầu.",
		Body: "Mỗi cảnh một số, luôn kèm mốc so sánh.",
		CTA:  "Điều con số đang nói với chúng ta.",
	},

	// ---------- Giải trí ----------
	{
		ID: "hai-huoc-tinh-huong", Name: "Hài tình huống", Icon: "😂", Category: CatGiaiTri,
		Desc:   "Tình huống đời thường bị đẩy lên mức buồn cười.",
		Script: "Viết kịch bản hài tình huống. Dựng một tình huống đời thường ai cũng gặp rồi đẩy dần tới mức vô lý. Điểm bật cười phải nằm ở cảnh CUỐI, không tiết lộ sớm. Câu thoại ngắn, đúng cách nói đời thường.",
		Style:  "Doodle 2D vẽ tay", Aspect: "9:16", Platform: "tiktok", Seconds: 30,
		VoicePace: "nhanh", MusicMood: "vui-tuoi",
		Hook: "Tình huống quen thuộc, chưa có gì lạ.",
		Body: "Đẩy dần tới mức vô lý.",
		CTA:  "Cú bật cười, ngắt ngay, không giải thích.",
	},
	{
		ID: "kinh-di-ngan", Name: "Chuyện kinh dị ngắn", Icon: "🕯", Category: CatGiaiTri,
		Desc:   "Chuyện rùng rợn 60 giây, sợ bằng gợi ý chứ không bằng máu me.",
		Script: "Viết chuyện kinh dị ngắn. Sợ đến từ điều KHÔNG nói ra và nhịp chậm dần, không từ mô tả ghê rợn. Giữ một chi tiết bất thường từ đầu, tới cuối mới cho biết nó nghĩa là gì. Không giải thích quá rõ ở câu cuối.",
		Style:  "Neon tương lai", Aspect: "9:16", Platform: "tiktok", Seconds: 60,
		VoiceGender: "Nữ", VoicePace: "chậm", MusicMood: "u-am",
		Hook: "Một chi tiết bình thường nhưng sai sai.",
		Body: "Nhịp chậm dần, chi tiết lạ lặp lại.",
		CTA:  "Cho biết chi tiết đó nghĩa là gì — rồi dừng.",
	},
	{
		ID: "the-gioi-vi-mo", Name: "Thế giới vi mô", Icon: "🔬", Category: CatGiaiTri,
		Desc:   "Phóng to những thứ mắt thường không thấy.",
		Script: "Kể về một thế giới rất nhỏ (côn trùng, vi sinh, tinh thể…) như kể về một hành tinh khác. Mỗi cảnh một sinh vật hoặc hiện tượng, luôn quy đổi kích thước sang thứ người xem hình dung được.",
		Style:  "Điện ảnh chân thực", Aspect: "9:16", Platform: "tiktok", Seconds: 45,
		VoicePace: "chậm", MusicMood: "huyen-ao",
		Hook: "Một sinh vật quen thuộc nhìn ở mức phóng to.",
		Body: "Mỗi cảnh một điều kỳ lạ, có quy đổi kích thước.",
		CTA:  "Điều này đang xảy ra ngay quanh ta.",
	},

	// ---------- Đời sống ----------
	{
		ID: "vlog-du-lich", Name: "Vlog du lịch", Icon: "✈️", Category: CatDoiSong,
		Desc:   "Một chuyến đi kể theo cảm giác, không theo lịch trình.",
		Script: "Viết lời dẫn vlog du lịch theo CẢM GIÁC chứ không theo lịch trình. Mỗi cảnh một khoảnh khắc cụ thể (mùi, tiếng động, một câu người địa phương nói) thay vì liệt kê địa danh. Tránh giọng hướng dẫn viên.",
		Style:  "Điện ảnh chân thực", Aspect: "9:16", Platform: "reels", Seconds: 45,
		VoicePace: "vừa", MusicMood: "nhe-nhang",
		Hook: "Khoảnh khắc đắt nhất của chuyến đi.",
		Body: "Các khoảnh khắc, không phải các địa điểm.",
		CTA:  "Điều còn đọng lại sau khi về.",
	},
	{
		ID: "suc-khoe-thuong-thuc", Name: "Sức khoẻ thường thức", Icon: "🩺", Category: CatDoiSong,
		Desc:   "Kiến thức sức khoẻ đơn giản, có ranh giới rõ.",
		Script: "Viết video sức khoẻ thường thức. Chỉ nói những điều phổ thông đã được đồng thuận rộng rãi. BẮT BUỘC có một câu nhắc đi khám khi có dấu hiệu bất thường, và KHÔNG chẩn đoán, KHÔNG kê đơn, KHÔNG hứa chữa khỏi.",
		Style:  "Phẳng tối giản", Aspect: "9:16", Platform: "shorts", Seconds: 45,
		VoicePace: "vừa", MusicMood: "nhe-nhang",
		Hook: "Một hiểu lầm phổ biến.",
		Body: "Điều đúng là gì, vì sao.",
		CTA:  "Khi nào thì phải đi khám.",
	},
	{
		ID: "meo-vat-hang-ngay", Name: "Mẹo vặt hàng ngày", Icon: "💡", Category: CatDoiSong,
		Desc:   "Một mẹo làm được ngay, không cần mua gì.",
		Script: "Viết video mẹo vặt. Đúng MỘT mẹo cho cả video, làm được ngay bằng đồ có sẵn trong nhà. Từng bước rõ ràng, mỗi cảnh một bước. Nói luôn mẹo này KHÔNG dùng được khi nào.",
		Style:  "Doodle 2D vẽ tay", Aspect: "9:16", Platform: "tiktok", Seconds: 30,
		VoicePace: "nhanh", MusicMood: "vui-tuoi",
		Hook: "Vấn đề vặt ai cũng gặp.",
		Body: "Từng bước, mỗi cảnh một bước.",
		CTA:  "Lưu ý khi nào không dùng được.",
	},

	// ---------- Kể chuyện ----------
	{
		ID: "tom-tat-phim", Name: "Tóm tắt phim", Icon: "🎬", Category: CatKeChuyen,
		Desc:   "Kể lại một bộ phim cho người chưa xem.",
		Script: "Kể lại một bộ phim cho người CHƯA xem: giữ mạch, giữ hồi hộp, không spoil đoạn kết ở giữa video. Gọi nhân vật bằng vai trò khi chưa cần tên. Nhịp dồn dần về cuối.",
		Style:  "Điện ảnh chân thực", Aspect: "9:16", Platform: "tiktok", Seconds: 60,
		VoicePace: "nhanh", MusicMood: "cang-thang",
		Hook: "Tình huống mở đầu phim, dựng thẳng vào.",
		Body: "Mạch diễn biến, giữ nút thắt.",
		CTA:  "Gợi mở kết, không nói toạc.",
	},
	{
		ID: "chuyen-co-tich", Name: "Chuyện kể dân gian", Icon: "🏮", Category: CatKeChuyen,
		Desc:   "Chuyện cổ, chuyện dân gian kể lại bằng giọng hiện đại.",
		Script: "Kể lại một câu chuyện dân gian bằng ngôn ngữ hiện đại nhưng giữ nguyên cốt và bài học. Không thêm bình luận đạo lý ở cuối — để câu chuyện tự nói.",
		Style:  "Doodle 2D vẽ tay", Aspect: "9:16", Platform: "tiktok", Seconds: 60,
		VoiceGender: "Nữ", VoicePace: "chậm", MusicMood: "huyen-ao",
		Hook: "Câu mở chuyện, đặt bối cảnh trong một câu.",
		Body: "Diễn biến, giữ nhịp kể.",
		CTA:  "Kết chuyện, không giảng giải.",
	},
	{
		ID: "podcast-cat-clip", Name: "Cắt podcast thành clip", Icon: "🎙", Category: CatKeChuyen,
		Desc:   "Lấy đoạn đắt nhất trong bản thu dài làm clip ngắn.",
		Script: "Chọn đoạn đắt giá nhất trong bản thu dài. Cắt bỏ mọi câu dẫn dắt vòng vo, giữ lại đúng ý mạnh nhất. Đưa câu chốt lên đầu làm mồi rồi mới quay lại mạch.",
		Style:  "Editorial 2D hiện đại", Aspect: "9:16", Platform: "tiktok", Seconds: 60,
		VoicePace: "vừa", MusicMood: "nhe-nhang",
		Hook: "Câu nói mạnh nhất, đưa lên đầu.",
		Body: "Quay lại mạch, giải thích ý đó.",
		CTA:  "Đóng lại bằng chính câu mở đầu.",
	},

	// ---------- Doanh nghiệp ----------
	{
		ID: "gioi-thieu-cong-ty", Name: "Giới thiệu công ty", Icon: "🏢", Category: CatDoanhNghi,
		Desc:   "Hồ sơ năng lực ngắn, nói bằng việc đã làm.",
		Script: "Viết video giới thiệu doanh nghiệp. Nói bằng VIỆC ĐÃ LÀM và con số, không bằng tính từ tự khen. Mỗi cảnh một bằng chứng cụ thể. Tránh mọi câu sáo rỗng kiểu 'uy tín hàng đầu'.",
		Style:  "Điện ảnh chân thực", Aspect: "16:9", Platform: "youtube", Seconds: 60,
		VoiceGender: "Nam", VoicePace: "vừa", MusicMood: "hung-trang",
		Hook: "Một con số hoặc thành tựu cụ thể.",
		Body: "Mỗi cảnh một bằng chứng.",
		CTA:  "Lời mời hợp tác rõ ràng.",
	},
	{
		ID: "gioi-thieu-repo", Name: "Giới thiệu dự án mã nguồn", Icon: "💻", Category: CatDoanhNghi,
		Desc:   "Giới thiệu một repo/công cụ cho dân kỹ thuật.",
		Script: "Giới thiệu một dự án phần mềm cho người làm kỹ thuật. Nói rõ nó GIẢI QUYẾT vấn đề gì, khác gì cái đang có, cài đặt ra sao. Không thổi phồng — nêu cả giới hạn.",
		Style:  "Phẳng tối giản", Aspect: "16:9", Platform: "youtube", Seconds: 60,
		VoicePace: "vừa", MusicMood: "nhe-nhang",
		Hook: "Vấn đề mà dân kỹ thuật đang phải chịu.",
		Body: "Cách dự án giải quyết, kèm giới hạn.",
		CTA:  "Link và cách bắt đầu.",
	},
	{
		ID: "tuyen-dung", Name: "Tuyển dụng", Icon: "📣", Category: CatDoanhNghi,
		Desc:   "Tin tuyển dụng dạng video, nói thật về công việc.",
		Script: "Viết video tuyển dụng. Nói thật về công việc: làm gì hằng ngày, khó ở đâu, hợp với người thế nào. Nêu quyền lợi cụ thể thay vì 'môi trường năng động'.",
		Style:  "Editorial 2D hiện đại", Aspect: "9:16", Platform: "reels", Seconds: 45,
		VoicePace: "vừa", MusicMood: "hao-hung",
		Hook: "Vị trí và điều hấp dẫn nhất của nó.",
		Body: "Công việc thật sự là gì, khó ở đâu.",
		CTA:  "Cách ứng tuyển.",
	},

	// ---------- thêm cho Quảng cáo ----------
	{
		ID: "mo-hop", Name: "Mở hộp sản phẩm", Icon: "📦", Category: CatQuangCao,
		Desc:   "Mở hộp theo trình tự, giữ tò mò tới món cuối.",
		Script: "Viết lời dẫn mở hộp. Theo đúng trình tự lấy đồ ra, mỗi cảnh một món, giữ món thú vị nhất tới cuối. Nhận xét ngắn ngay khi cầm lên, đừng chờ tổng kết.",
		Style:  "Điện ảnh chân thực", Aspect: "9:16", Platform: "tiktok", Seconds: 45,
		VoicePace: "nhanh", MusicMood: "vui-tuoi",
		Hook: "Chiếc hộp còn nguyên seal.",
		Body: "Từng món, nhận xét ngay.",
		CTA:  "Món đáng tiền nhất trong hộp.",
	},
	{
		ID: "khuyen-mai-flash", Name: "Thông báo khuyến mãi", Icon: "🔥", Category: CatQuangCao,
		Desc:   "Video ngắn báo deal, nhấn thời hạn.",
		Script: "Viết video thông báo khuyến mãi thật ngắn. Nói ngay MỨC giảm và HẠN chót trong 3 giây đầu. Mỗi cảnh một thông tin, không nhồi. Không phóng đại con số.",
		Style:  "Neon tương lai", Aspect: "9:16", Platform: "tiktok", Seconds: 15,
		VoicePace: "nhanh", MusicMood: "hao-hung",
		Hook: "Mức giảm và hạn chót.",
		Body: "Áp dụng cho gì, điều kiện gì.",
		CTA:  "Mua ở đâu, ngay bây giờ.",
	},
	{
		ID: "cam-nhan-khach-hang", Name: "Cảm nhận khách hàng", Icon: "💬", Category: CatQuangCao,
		Desc:   "Lời khách thật, dựng thành video tin được.",
		Script: "Dựng video từ cảm nhận khách hàng. Giữ nguyên cách nói của họ, kể cả chỗ vụng — sửa cho trau chuốt là mất tin. Mỗi cảnh một ý của một người. Có bối cảnh cụ thể (dùng bao lâu, dùng để làm gì).",
		Style:  "Điện ảnh chân thực", Aspect: "9:16", Platform: "reels", Seconds: 45,
		VoicePace: "vừa", MusicMood: "nhe-nhang",
		Hook: "Câu nhận xét thẳng thắn nhất.",
		Body: "Từng người, từng bối cảnh cụ thể.",
		CTA:  "Mời thử, không ép.",
	},
	{
		ID: "huong-dan-su-dung", Name: "Hướng dẫn sử dụng", Icon: "🔧", Category: CatGiaoDuc,
		Desc:   "Chỉ cách dùng một thứ, từng bước không bỏ sót.",
		Script: "Viết hướng dẫn sử dụng từng bước. Mỗi cảnh MỘT thao tác, nói rõ nhìn thấy gì sau khi làm xong bước đó. Nêu trước những thứ cần chuẩn bị. Cảnh báo bước dễ sai.",
		Style:  "Phẳng tối giản", Aspect: "9:16", Platform: "shorts", Seconds: 60,
		VoicePace: "vừa", MusicMood: "nhe-nhang",
		Hook: "Kết quả cuối cùng sẽ đạt được.",
		Body: "Từng bước, mỗi cảnh một thao tác.",
		CTA:  "Bước dễ sai nhất cần lưu ý.",
	},
}

// All trả toàn bộ khuôn (bản sao).
func All() []Template {
	out := make([]Template, len(all))
	copy(out, all)
	return out
}

// Find tìm khuôn theo id.
func Find(id string) (Template, bool) {
	for _, t := range all {
		if strings.EqualFold(t.ID, id) {
			return t, true
		}
	}
	return Template{}, false
}

// Categories trả danh mục theo đúng thứ tự xuất hiện.
func Categories() []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range all {
		if !seen[t.Category] {
			seen[t.Category] = true
			out = append(out, t.Category)
		}
	}
	return out
}
