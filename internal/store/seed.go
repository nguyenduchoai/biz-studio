package store

// seedPrompts nạp bộ prompt mẫu lần đầu (chỉ khi chưa từng seed và danh sách trống).
func (s *Store) seedPrompts() {
	if s.d.Seeded || len(s.d.Prompts) > 0 {
		return
	}
	for _, p := range defaultPrompts {
		p.ID = s.NewID("pmt")
		s.d.Prompts = append(s.d.Prompts, p)
	}
	s.d.Seeded = true
	_ = s.saveLocked()
}

var defaultPrompts = []PromptTemplate{
	{Name: "🔥 Video ngắn viral (TikTok/Reels)", Body: "Edit thành video ngắn dọc 9:16 tối đa 60 giây, nhịp nhanh.\n- 3 giây đầu phải là hook mạnh nhất (chọn khoảnh khắc đắt giá nhất đưa lên đầu).\n- Cắt bỏ toàn bộ khoảng lặng, câu thừa, đoạn lan man.\n- Phụ đề to rõ giữa màn hình, highlight keyword chính bằng màu nổi.\n- Sound effect nhẹ khi chuyển ý (whoosh/pop), không chói tai.\n- Kết thúc bằng câu chốt hoặc câu hỏi giữ tương tác, không để video trôi tự do."},
	{Name: "🛍 Giới thiệu sản phẩm (TVC ngắn)", Body: "Dựng TVC giới thiệu sản phẩm theo đúng thứ tự và mô tả của từng asset.\n- Tông sang trọng, chuyển cảnh mượt (crossfade/zoom nhẹ), giữ màu sắc trung thực của sản phẩm.\n- Mở đầu bằng cận cảnh điểm nổi bật nhất, giữa video nêu 2–3 lợi ích chính, cuối video là logo + CTA.\n- Chèn ảnh sản phẩm đúng ngữ cảnh được mô tả trong từng asset, mỗi ảnh hiển thị 2–3 giây.\n- Nhạc nền hiện đại, âm lượng nhỏ hơn giọng đọc.\n- Chữ trên màn hình tối giản, chỉ tên sản phẩm + thông số đắt giá."},
	{Name: "✈️ Vlog du lịch", Body: "Edit theo phong cách vlog du lịch thư giãn.\n- Giữ âm thanh môi trường (sóng, gió, phố xá) làm nền, nhạc nền nhẹ không lấn.\n- Màu ấm, tươi; nếu footage quay log thì delog và thêm LUT màu phim.\n- Chuyển cảnh mượt theo chuyển động máy, ưu tiên cắt theo nhịp nhạc.\n- Cắt bỏ đoạn rung lắc mạnh, đoạn đứng yên quá 4 giây.\n- Phụ đề chỉ hiện ở câu thoại quan trọng, kiểu chữ mảnh, đặt thấp."},
	{Name: "🎓 Video kiến thức / giáo dục", Body: "Edit thành video kiến thức dễ theo dõi, KHÔNG cắt mất ý.\n- Giữ đầy đủ luận điểm; chỉ cắt khoảng lặng và từ đệm (ừm, à...).\n- Tạo phụ đề karaoke chính xác từng từ, xuất kèm file .srt.\n- Phân tích nội dung và highlight 3–5 keyword quan trọng nhất (phóng to hoặc đổi màu khi nhắc tới).\n- Chèn ảnh minh họa đúng lúc khái niệm được nhắc đến, mỗi ảnh 3–4 giây.\n- Tiết tấu vừa phải, không dồn dập; giữ nguyên thứ tự trình bày gốc."},
	{Name: "🎤 Recap sự kiện", Body: "Dựng video recap sự kiện có cao trào.\n- Mở đầu bằng khoảnh khắc bùng nổ nhất làm hook (3–5 giây), sau đó mới theo dòng thời gian.\n- Cắt nhanh theo nhịp nhạc nền sôi động; mỗi cảnh 1.5–3 giây.\n- Chèn tiêu đề sự kiện + ngày tháng ở đầu, tên hoạt động ở từng phân đoạn.\n- Âm thanh: ưu tiên tiếng vỗ tay/reo hò thật ở các đoạn cao trào, nhạc nền dẫn dắt phần còn lại.\n- Kết bằng cảnh toàn thể + lời cảm ơn/CTA."},
	{Name: "💻 Giới thiệu repo / công nghệ", Body: "Edit video giới thiệu công nghệ phong cách tech-demo tối giản.\n- Bố cục: vấn đề → giải pháp → demo → kết quả → CTA (star repo/dùng thử).\n- Chèn screenshot/ảnh theo đúng thứ tự asset, mỗi ảnh giữ đủ lâu để đọc được (3–5 giây).\n- Chữ trên màn hình to, rõ, nền tối; highlight tên công nghệ và con số ấn tượng.\n- Không cần phụ đề karaoke nếu đã có chữ trên màn hình; sound effect gõ phím/click nhẹ khi chuyển demo.\n- Giữ nguyên thời lượng tổng thể gọn dưới 90 giây."},
	{Name: "🎙 Cắt podcast thành clip", Body: "Từ bản ghi dài, chọn ra 1–3 đoạn đắt giá nhất (insight mạnh, câu nói gây tranh luận, câu chuyện cảm động) dựng thành clip dọc 9:16 dưới 90 giây.\n- Mỗi clip phải tự đứng được: có mở – thân – kết, không cần ngữ cảnh ngoài.\n- Phụ đề karaoke từng từ, keyword chính đổi màu.\n- Cắt sạch khoảng lặng và từ đệm; giữ tự nhiên, không cắt gãy câu.\n- Thêm tên người nói + chủ đề ở góc trên trong 5 giây đầu."},
	{Name: "📊 Video số liệu / báo cáo", Body: "Dựng video trình bày số liệu rõ ràng, đáng tin.\n- Mỗi con số quan trọng được phóng to chiếm màn hình 2–3 giây kèm đơn vị và nguồn.\n- Chart/bảng phải hiển thị đủ lâu để đọc (tối thiểu 4 giây), zoom vào phần đang nói tới.\n- Giọng đọc chậm, rõ; phụ đề đầy đủ.\n- Màu sắc nhất quán: 1 màu chủ đạo cho số liệu chính, 1 màu phụ cho so sánh.\n- Kết video bằng slide tổng kết 3 ý chính."},
}
