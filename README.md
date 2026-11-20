# ⚡ VelocityEngine v2 RTX — Studio Biên Tập Video CUDA Zero-Copy

**VelocityEngine v2 RTX** là ứng dụng biên tập & tăng tốc xuất video cục bộ trên Windows, kết hợp **Go 1.26 (`net/http` + SSE)**, **SvelteKit 5 + Konva.js + Interact.js**, và bộ engine C++ **100% CUDA NVRTC Zero-Copy** (`velocity-cuda.exe`) chạy trực tiếp trên GPU NVIDIA.

---

## 🛠️ Tính năng nổi bật
* **100% Zero-Copy CUDA Pipeline (`velocity-cuda.exe`)**: Giải mã phần cứng **NVDEC** (`H.264`, `HEVC`, `AV1`) → Lọc màu BT.709 & trộn Alpha Overlay PNG ngay trên VRAM bằng **CUDA NVRTC** → **NVENC** (`h264_nvenc` / `hevc_nvenc`), không copy khung hình qua CPU RAM.
* **Chất lượng hình ảnh Pure `CQ 20` + `spatial_aq = 1`**: Khóa mặc định chế độ nét cao, không bóp trần bitrate `-maxrate`, giữ nguyên độ nét chi tiết (SSIM `~0.998`).
* **Hỗ trợ đầy đủ NLE Timeline**:
  * **Biên tập**: Multi-track timeline (`interactjs`), 5 bộ lọc màu (`grayscale`, `sepia`, `warm`, `cool`, `vibrant`), thêm chữ/ảnh phủ WYSIWYG (Konva rasterized PNG), cắt nhiều đoạn (`trim`), đổi tốc độ `0.25x–4x` kèm đồng bộ âm thanh `atempo`.
  * **Chia & Ghép video**: Chia theo mốc hoặc chia đều nhiều phần (`/api/split`), ghép nhiều clip khác độ phân giải (`/api/merge`).
  * **Hỗ trợ tên file Tiếng Việt UTF-8 đầy đủ** và hệ thống dự phòng tự động (**CUDA NVRTC** → **Vulkan `libplacebo` + NVENC** → **CPU**).
* **Đóng gói 1 file duy nhất (`velocity.exe`)**: Nhúng sẵn giao diện SvelteKit build tĩnh vào binary Go (`CGO_ENABLED=0`), đi kèm System Tray Windows và SSE telemetry (`fps`, `speed`, `ETA`, console stream).

---

## 📦 Link tải FFmpeg & Công cụ / SDK cần chuẩn bị (Windows)

Repo Git không đính kèm file thực thi (`.exe`) hay thư viện ngoài. Để build và chạy đầy đủ chế độ **CUDA Zero-Copy**, hãy tải các gói sau (khuyên đặt gọn vào một thư mục như `D:\code\environment\`):

| Thành phần | Mục đích sử dụng | Link tải chính thức | Đường dẫn mặc định trong code |
| :--- | :--- | :--- | :--- |
| **FFmpeg Full (`release-full`)** | Chạy `ffmpeg.exe` / `ffprobe.exe` (Vulkan fallback, Probe, Split/Merge) | [gyan.dev/ffmpeg/builds](https://www.gyan.dev/ffmpeg/builds/) (`ffmpeg-release-full.7z`) | Thêm thư mục `bin\` vào `PATH` (vd: `D:\code\environment\ffmpeg\bin`) |
| **FFmpeg Shared SDK (`release-full-shared`)** | Cung cấp `include/`, `lib/`, và DLL (`avcodec-63.dll`...) để biên dịch & chạy `velocity-cuda.exe` | [gyan.dev/ffmpeg/builds](https://www.gyan.dev/ffmpeg/builds/) (`ffmpeg-release-full-shared.7z`) | `D:\code\environment\ffmpeg-shared-9.0.2` |
| **NVIDIA CUDA 12.8 (`nvrtc`, `cudart`, `cccl`)** | Header `cuda.h` và DLL `nvrtc64_120_0.dll` biên dịch kernel CUDA lúc chạy (cần Driver NVIDIA hỗ trợ CUDA) | [NVIDIA CUDA Toolkit / Redist](https://developer.nvidia.com/cuda-downloads) (`12.8`) | `D:\code\environment\cuda-12.8` |
| **MinGW-w64 (`g++` C++17)** | Biên dịch `cuda_render/main.cpp` thành `velocity-cuda.exe` | [winlibs.com](https://winlibs.com/) | `D:\code\environment\mingw64` |
| **Go SDK (`>= 1.26`)** | Biên dịch Backend & đóng gói `velocity.exe` | [go.dev/dl](https://go.dev/dl/) | `D:\code\environment\GoSDK\GoSDK-1.26.2` (hoặc `go` trong `PATH`) |
| **Node.js (`>= 22`)** | Build frontend SvelteKit, chạy Vitest & Playwright | [nodejs.org](https://nodejs.org/) | `node` / `npm` trong `PATH` |

> **Lưu ý khi cài mới**: Nếu bạn đặt FFmpeg Shared / CUDA / MinGW ở đường dẫn khác `D:\code\environment\...`, hãy cập nhật lại đường dẫn tương ứng trong `build.ps1`, `cuda_render/main.cpp`, `cuda_render/cuda_color.hpp` và `backend/internal/ffmpeg/cuda_runner.go`.

---

## 🚀 Cài đặt, Build & Chạy ứng dụng

### 1. Cài dependencies Frontend (chỉ cần chạy lần đầu)
```powershell
cd frontend
npm install
npx playwright install chromium   # Nếu muốn chạy test E2E trình duyệt
cd ..
```

### 2. Build ra `velocity-cuda.exe` và `velocity.exe`
```powershell
# Chỉ build binary
.\build.ps1

# Build + chạy Go Unit Tests, Go Real-GPU CUDA E2E Tests, Vitest & svelte-check
.\build.ps1 -Test

# Chạy toàn bộ -Test + Playwright Full Browser E2E (cổng 18089)
.\build.ps1 -Test -E2E
```

### 3. Khởi chạy
Click đúp vào `velocity.exe` (hoặc chạy `.\velocity.exe` trong terminal). App sẽ tự mở giao diện Web Studio tại `http://127.0.0.1:8080/` (hoặc cổng trống kế tiếp `8080–8099`).

> **Không bấm trực tiếp `velocity-cuda.exe`**: File này là worker CLI được `velocity.exe` gọi ngầm kèm biến môi trường `PATH` trỏ tới FFmpeg Shared & CUDA DLL.

---

## 📁 Cấu trúc thư mục
* `cuda_render/`: Engine C++ CUDA NVRTC Zero-Copy (`main.cpp`, `cuda_color.hpp`, `cuda_overlay.hpp`, `ff_audio.hpp`).
* `backend/`: Server Go (`cmd/velocity`), điều phối FFmpeg/CUDA (`internal/ffmpeg`), API & Real-GPU E2E tests (`internal/server`), toán học timeline/geometry (`internal/mathutil`).
* `frontend/`: Giao diện SvelteKit 5 + Konva.js + Tailwind (`src/`) và bộ test Playwright (`tests/`).
* `build.ps1`: Script build & test hợp nhất một chạm.
