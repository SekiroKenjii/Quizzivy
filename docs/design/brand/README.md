# Quizzivy — Bộ nhận diện thương hiệu (SVG)

Toàn bộ file vector được dựng lại chính xác theo thiết kế gốc: vòng chữ Q than chì,
dải ruy băng xanh lá đan xen (luồn dưới vòng ở góc trên phải, vắt qua vòng ở góc dưới
phải) và phần gập giấy màu xám. Vòng tròn được dựng bằng đường tròn hình học chuẩn,
các khe trắng đồng tâm bề rộng đồng nhất.

## Màu thương hiệu

| Vai trò              | Hex       |
|----------------------|-----------|
| Xanh lá (chính)      | `#a4ca3e` |
| Than chì (chính)     | `#353d3f` |
| Xám (phụ)            | `#848787` |
| Xám trên nền tối     | `#a7abad` |

## Cấu trúc thư mục

### `svg/` — file vector gốc (scale vô hạn, nền trong suốt)
- `quizzivy-mark-*.svg` — biểu tượng đơn: `color`, `black` (in 1 màu đen), `white`
  (in 1 màu trắng), `on-dark` (bản màu dùng trên nền tối: vòng trắng + xanh lá + xám sáng).
- `quizzivy-logo-horizontal-*.svg` — logo ngang có chữ: `color`, `black`, `white`, `on-dark`.
- `quizzivy-logo-vertical-*.svg` — logo dọc có chữ: `color`, `white`, `on-dark`.
- `quizzivy-appicon-light.svg` / `quizzivy-appicon-dark.svg` — icon ứng dụng 1024×1024, bo góc 229.
- `quizzivy-favicon.svg` — bản vuông cho favicon (trình duyệt hiện đại dùng SVG trực tiếp).

### `png/` — bản xuất sẵn tiện dùng
Mark 1024px, logo ngang 2400px, logo dọc 1200px, app icon 1024px, favicon 512/192/32px.

## Ghi chú sử dụng
- Nền sáng → dùng bản `color`; nền tối → dùng `on-dark` hoặc `white`; in ấn 1 màu → `black`/`white`.
- Phần chữ dùng font **Quicksand Bold** (giấy phép mở SIL OFL) và đã được chuyển thành
  path — hiển thị đúng ở mọi nơi, không cần cài font.
- Kích thước tối thiểu khuyến nghị: mark ≥ 24px chiều cao; logo ngang ≥ 120px chiều rộng.
