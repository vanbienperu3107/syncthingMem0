# syncthingMem0 — CLIENT

Bản cài cho **máy cá nhân** kết nối tới hub qua WSS:443. Trình cài tự sinh khóa
thiết bị + cấu hình (không phải tự tay sửa config).

## Nội dung gói
- `syncthing` / `syncthing.exe` — binary client
- `install.sh` (Linux/macOS) hoặc `install.ps1` (Windows) — trình cài tự cấu hình
- `README.md` — tài liệu này

## Cài đặt

**Linux / macOS**
```sh
chmod +x install.sh
HUB_URL=vps.example.com:443 HUB_DEVICE_ID=<ID-HUB> HUB_TOKEN=<JWT> ./install.sh
# hoặc chạy không tham số để nhập tương tác:
./install.sh
```

**Windows (PowerShell)**
```powershell
$env:HUB_URL="vps.example.com:443"; $env:HUB_DEVICE_ID="<ID-HUB>"; $env:HUB_TOKEN="<JWT>"
powershell -ExecutionPolicy Bypass -File install.ps1
```

## Tham số (biến môi trường)
| Biến | Bắt buộc | Mặc định | Ý nghĩa |
|------|----------|----------|---------|
| `HUB_URL` | ✅ | — | `host:443` của hub |
| `HUB_DEVICE_ID` | ✅ | — | Device ID của hub (để ghép cặp) |
| `HUB_TOKEN` | — | — | JWT do hub cấp qua `/api/register` |
| `FOLDER_PATH` | — | `~/SyncMem0` | Thư mục được đồng bộ |
| `FOLDER_ID` | — | `default` | ID thư mục (phải khớp hub) |
| `BIN_DIR` | — | `~/.local/bin` (Win: `%LOCALAPPDATA%\syncthingMem0`) | Nơi đặt binary |
| `STHOMEDIR` | — | `~/.config/syncthingmem0` | Thư mục cấu hình |

## Sau khi cài
1. Trình cài in ra **Device ID của máy này** → vào hub authorize device đó vào folder.
2. Chạy client: `STHOMEDIR=<config> syncthingmem0 serve`.

> Lưu ý: hiện tại định danh kết nối vẫn dựa trên chứng chỉ TLS của thiết bị (device ID),
> JWT mới dùng cho REST API. Xem `docs/10-SOLUTION-REVIEW.md` (§6) để biết chi tiết.
