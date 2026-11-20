# ⚡ VelocityEngine RTX - GPU Video Speed Accelerator

VelocityEngine RTX là ứng dụng thay đổi tốc độ video (1.2x, 1.3x, hoặc tùy chọn) được tối ưu hóa phần cứng bằng cách tận dụng card đồ họa **NVIDIA GeForce RTX 3060** của bạn thông qua bộ giải mã/mã hóa phần cứng **NVENC** của FFmpeg.

Ứng dụng kết hợp giữa một Backend Script (Python) mạnh mẽ và giao diện Giám sát Tiến trình (HTML/CSS/JS) hiện đại.

---

## 🛠️ Tính năng nổi bật
* **GPU Hardware Encoding (`h264_nvenc`)**: Mã hóa video trực tiếp trên nhân đồ họa RTX 3060 giúp render siêu tốc và giảm tải tối đa cho CPU.
* **Giao diện Giám sát Premium (Dark/Neon Glassmorphism)**: Theo dõi tiến độ thời gian thực (%), FPS render, tốc độ xử lý thực tế, thời gian đã chạy và thời gian dự kiến hoàn thành (ETA).
* **Đồng bộ hóa âm thanh nâng cao**: Tự động chia nhỏ/chuỗi các bộ lọc âm thanh `atempo` nếu bạn tăng tốc độ lên quá cao (>2x) hoặc hạ quá thấp (<0.5x) mà không làm mất tiếng.
* **Nhật ký FFmpeg thời gian thực**: Stream trực tiếp các dòng log từ tiến trình FFmpeg chạy ngầm lên giao diện Web để theo dõi chi tiết.
* **Hỗ trợ 2 phương thức tiện lợi**:
  1. **Upload Video trực tiếp**: Kéo thả file video (< 500MB) vào giao diện web (tải trực tiếp lên localhost chỉ mất 0.5s).
  2. **Dán đường dẫn tuyệt đối (Local Path)**: Tiện lợi và nhanh gọn cho các file video dung lượng lớn trên ổ cứng.

---

## 🚀 Cách chạy ứng dụng

### Bước 1: Khởi động Server Python
Mở terminal tại thư mục dự án và chạy lệnh sau:
```powershell
python app.py
```

### Bước 2: Thao tác trên Web
1. Sau khi khởi động, trình duyệt sẽ tự động mở trang web giao diện tại địa chỉ: `http://localhost:8080/` (hoặc cổng tiếp theo nếu 8080 bị trùng).
2. Chọn phương thức xử lý (kéo thả file hoặc dán đường dẫn).
3. Thiết lập tốc độ mong muốn (các mốc nhanh 1.2x, 1.3x... hoặc dùng thanh trượt kéo chọn tùy ý).
4. Đảm bảo nút **GPU Acceleration** đang được bật để kích hoạt sức mạnh RTX 3060.
5. Bấm **ACCELERATE PROCESSING**.
6. Xem tiến độ render trực tiếp. Khi hoàn thành, bạn có thể **xem thử video đã xử lý** ngay trên web và nhấn **DOWNLOAD** để lưu về máy.

---

## 📁 Cấu trúc thư mục
* `app.py`: Backend server xử lý API, điều phối FFmpeg và parse progress.
* `index.html`: Layout cấu trúc giao diện điều khiển.
* `index.css`: Styling Glassmorphism/Dark mode cho trang web.
* `index.js`: Logic gửi file, gọi API, polling status và hiển thị logs.
* `uploads/`: Thư mục lưu trữ tạm các video được upload lên.
* `downloads/`: Thư mục lưu trữ các video đầu ra sau khi xử lý thành công.
