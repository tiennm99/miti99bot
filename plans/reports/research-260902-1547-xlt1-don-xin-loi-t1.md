# Research Report: Văn mẫu "Đơn xin lỗi T1" cho lệnh `/xlt1`

Thời điểm nghiên cứu: 2026-09-02 15:47 (+07)

## Executive Summary

Meme "đơn xin lỗi" của Việt Nam là parody **đơn từ hành chính**: giữ nguyên bố cục
công văn nhà nước (quốc hiệu, "Kính gửi", "Tôi tên là", "Nội dung sự việc",
"Tôi xin cam kết", khối ký tên) nhưng nội dung là chuyện tầm phào. Tiếng cười đến
từ *độ vênh* giữa hình thức trang trọng và nội dung vô nghĩa — không đến từ câu
chữ chửi bới. Đây là phát hiện then chốt: văn mẫu `/xlt1` phải **giống công văn
thật**, không phải một đoạn rant nữa.

Ngữ cảnh T1: cộng đồng LMHT Việt gọi T1 là "Tếu 1", dùng meme "tê liệt ngồi xe lăn"
(tồn tại từ khi SKT T1 đổi tên thành T1, cả fan lẫn anti đều dùng). Motif "trù xong
phải quay xe" là trung tâm — người viết đơn không phải anti thuần, mà là **fan đã
mất niềm tin sớm** rồi T1 lật kèo.

Khuyến nghị: `/xlt1` là **hồi tiếp nối `/ff`** đã có trong repo. `/ff` = "tắt stream
đi Tê Con ơi"; `/xlt1` = xin lỗi vì đã tắt sớm, đã trù. Cùng một nhân vật, hai thời
điểm. Cách này vừa tái dùng dàn punchline maintainer đã chốt trong `/ff`, vừa cho
lệnh mới một lý do tồn tại thay vì trùng thể loại.

## Research Methodology

- Sources consulted: 3 web_search (tiếng Việt) + đọc repo (`internal/modules/misc/ff_command.go`)
- Key terms: `"đơn xin lỗi T1" văn mẫu meme`, `"xin lỗi T1" ... "Tếu 1" Faker quay xe`, `"đơn xin lỗi" "kính gửi" T1 Faker CKTG "cam kết"`
- Giới hạn: web_search US-only; meme này sống chủ yếu trên TikTok/Facebook/Threads VN nên
  không truy được một bản "văn mẫu gốc" chuẩn. Bù lại bằng bố cục đơn hành chính VN
  (kiến thức ổn định) + dàn joke đã có trong repo.

## Key Findings

### 1. Bố cục "đơn xin lỗi" chuẩn (khung parody)

```
CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM
Độc lập – Tự do – Hạnh phúc
------------------

ĐƠN XIN LỖI

Kính gửi: <đối tượng>
Tôi tên là: <người viết>
Địa chỉ / Chức vụ: <...>

Nội dung sự việc: <thừa nhận sai>
Nay tôi làm đơn này để <mục đích>
Tôi xin cam kết: <1..n điều>
Kính mong <đối tượng> xem xét, tha thứ.
Tôi xin trân trọng cảm ơn!

                    Người làm đơn
                    (Ký và ghi rõ họ tên)
```

Nguồn hài: giữ **đủ** các mục này. Bỏ mục nào là mất chất công văn.

### 2. Kho joke T1 của cộng đồng Việt (đã có trong repo, tái dùng được)

Từ comment của `ffTemplate` — maintainer đã chốt các motif này:

| Motif | Nghĩa |
|---|---|
| "Tếu 1" | biệt danh chọc T1 tấu hài |
| "rạp xiếc trung ương" / "di sản văn hóa hài kịch của LCK" | T1 đá lỗi hài |
| "gói Premium hết hạn tháng 11" | T1 chỉ đỉnh vào mùa CKTG |
| "vía" | fan làm nghi thức cầu vía |
| "Tê Con" | biệt danh thân mật, chính T1 cũng dùng |

Bổ sung từ search: meme "tê liệt / ngồi xe lăn" (fan lẫn anti dùng nhiều năm).

### 3. Giọng điệu nên tránh

- Không chửi cầu thủ theo tên (`/ff` cũng cố tình tránh: "không cần soi ai riêng đâu").
- Không khẳng định số cúp / kết quả giải cụ thể → dữ kiện thay đổi theo mùa, template
  tĩnh sẽ lỗi thời. Nói "sự thật đã chứng minh tôi sai" là đủ, đúng mọi mùa.
- Không toxic thật; đây là joke nội bộ nhóm chat.

### 4. Quyết định thiết kế lệnh

| Hạng mục | Chọn | Lý do |
|---|---|---|
| Tên | `xlt1` | khớp `^[a-z0-9_]{1,32}$` (`internal/modules/validate.go:10`) |
| Module | `misc` | cùng chỗ với `/ff`, `/tth` |
| Visibility | `Protected` | soi chiếu `/ff` — cùng thể loại văn mẫu spam nhóm |
| Nội suy | mention người gửi ×2 | dùng lại `senderMention()` sẵn có (DRY); lấp ô "Tôi tên là" + "Người làm đơn" |
| Parse mode | HTML | mention cần thẻ `<a href="tg://user?id=...">`; giống `disclaimerCommand` |
| Tham số | không | như `/ff`, args bị bỏ qua |

## Implementation Recommendations

File mới `internal/modules/misc/xlt1_command.go`, một `const` template + một
`xlt1Command()`; đăng ký trong `New()` ngay sau `ffCommand()`. Test soi chiếu
`TestFF_*` trong `handlers_test.go`; cập nhật map trong
`TestNew_RegistersExpectedCommands`; cập nhật bảng module ở `README.md:11`.

### Common Pitfalls

- Template chèn qua `ReplyHTML` → tuyệt đối không để `<`, `&`, `>` thô trong văn mẫu,
  không thì Telegram trả 400. Dùng `–`, `oOo`, `...` thay vì `<...>`.
- `senderMention` đã `html.EscapeString` phần tên; target/prose tĩnh thì tự lo.

## Resources & References

- [Bộ sưu tập meme xin lỗi](https://yeuvanhoc.edu.vn/meme-xin-loi/) — thể loại "đơn xin lỗi" như công văn parody
- [Mẫu Đơn Xin Lỗi Meme](https://theselfishmeme.co.uk/don-xin-loi-meme) — biến thể bố cục
- [Threads: meme tê liệt xe lăn T1](https://www.threads.com/@ghuyph_/post/DQtlGBYj-gm/) — lịch sử meme, fan lẫn anti dùng
- [Faker – chức vô địch thứ 4](https://bytuanhuynh.substack.com/p/faker-chuc-vo-ich-thu-4-inh-cao-thich) — motif "hết thời rồi lật kèo"
- Repo: `internal/modules/misc/ff_command.go` — dàn punchline đã chốt

## Unresolved Questions

1. `/xlt1` nên `Protected` (soi `/ff`) hay `Public` (soi `/tth`)? Đã chọn `Protected`; đổi 1 dòng nếu muốn public.
2. Có cần alias (`/xinloit1`)? Hiện chỉ làm đúng `/xlt1` như yêu cầu.
