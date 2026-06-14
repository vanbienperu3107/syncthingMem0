# Hướng dẫn CI/CD tự động cho syncthingMem0

> Tài liệu này viết để **đọc là hiểu**, kể cả khi bạn mới làm quen CI/CD.
> Mỗi khái niệm đều được giải thích trước, rồi mới áp dụng vào dự án.

---

## Mục lục

1. [CI/CD là gì? Vì sao cần?](#1-cicd-là-gì-vì-sao-cần)
2. [GitHub Actions – các khái niệm nền tảng](#2-github-actions--các-khái-niệm-nền-tảng)
3. [Bức tranh tổng thể: push → build → release](#3-bức-tranh-tổng-thể-push--build--release)
4. [Giải thích chi tiết file `auto-release.yml`](#4-giải-thích-chi-tiết-file-auto-releaseyml)
5. [Vòng đời một lần chạy thực tế](#5-vòng-đời-một-lần-chạy-thực-tế)
6. [Cách xem kết quả và tải bản build](#6-cách-xem-kết-quả-và-tải-bản-build)
7. [Các workflow khác trong repo & vì sao đã “tắt”](#7-các-workflow-khác-trong-repo--vì-sao-đã-tắt)
8. [Tùy biến pipeline theo nhu cầu](#8-tùy-biến-pipeline-theo-nhu-cầu)
9. [Xử lý sự cố thường gặp](#9-xử-lý-sự-cố-thường-gặp)
10. [Phụ lục: vì sao phải “generate assets”](#10-phụ-lục-vì-sao-phải-generate-assets)

---

## 1. CI/CD là gì? Vì sao cần?

- **CI = Continuous Integration (Tích hợp liên tục):** mỗi lần bạn đẩy code lên,
  máy chủ sẽ **tự động kiểm tra** (build thử, chạy test) để phát hiện lỗi sớm.
- **CD = Continuous Delivery/Deployment (Giao hàng/Triển khai liên tục):** sau khi
  build thành công, hệ thống **tự động đóng gói và phát hành** sản phẩm (ở đây là
  file chạy `syncthing` cho nhiều hệ điều hành) mà bạn **không cần build trên máy cá nhân**.

**Lợi ích cụ thể cho bạn:**

- Không phải cài Go, không phải build local. Chỉ cần `git push`.
- Máy chủ của GitHub build hộ cho **Windows, Linux, macOS** cùng lúc.
- Mỗi lần push tạo ra một **bản Release tải về được**, có lịch sử rõ ràng.

---

## 2. GitHub Actions – các khái niệm nền tảng

GitHub Actions là hệ thống CI/CD **miễn phí** tích hợp sẵn trong GitHub. Bạn mô tả
công việc bằng file `.yml` đặt trong thư mục `.github/workflows/`. Các khái niệm:

| Khái niệm | Giải thích dễ hiểu | Ví dụ trong dự án |
|---|---|---|
| **Workflow** | Một “quy trình” hoàn chỉnh, là 1 file `.yml`. | `auto-release.yml` |
| **Trigger** (`on:`) | Sự kiện kích hoạt workflow. | `push` lên nhánh `main` |
| **Job** | Một “công việc lớn”, chạy trên 1 máy ảo riêng. | job `build`, job `release` |
| **Runner** | Máy ảo do GitHub cấp để chạy job. | `ubuntu-latest` |
| **Step** | Một bước nhỏ trong job, chạy lần lượt từ trên xuống. | “Build binary” |
| **Action** | Một “plugin” dùng lại, viết sẵn bởi người khác. | `actions/checkout` |
| **Matrix** | Nhân bản 1 job thành nhiều biến thể. | build cho 5 cặp HĐH/CPU |
| **Artifact** | File tạm do job tạo ra, các job khác tải lại được. | binary đã build |
| **Release** | Bản phát hành chính thức trên GitHub, kèm file tải về. | `v0.1.0-build.N` |
| **Secret** | Biến bí mật (mật khẩu, token) lưu an toàn trong repo. | (chưa cần) |
| **`GITHUB_TOKEN`** | Token tự sinh cho mỗi lần chạy, để workflow thao tác với repo. | dùng để tạo Release |
| **`permissions`** | Cấp quyền cho `GITHUB_TOKEN`. | `contents: write` để tạo Release |

**Quan hệ phân cấp:** `Workflow` ⟶ chứa nhiều `Job` ⟶ mỗi Job chứa nhiều `Step`.
Các Job mặc định **chạy song song**; muốn job B chờ job A xong thì dùng `needs: A`.

---

## 3. Bức tranh tổng thể: push → build → release

Pipeline tự động của dự án nằm trong **một file duy nhất**:
[`.github/workflows/auto-release.yml`](../.github/workflows/auto-release.yml).

Luồng hoạt động:

```
Bạn: git push lên nhánh main
          │
          ▼
GitHub phát hiện sự kiện "push" ──► khởi động workflow "Auto Release"
          │
          ▼
   JOB "build" (chạy song song 5 bản):
     linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64
        ├─ checkout code
        ├─ cài Go
        ├─ generate GUI assets
        ├─ go build (biên dịch ra file chạy)
        └─ upload artifact (cất binary tạm)
          │
          ▼  (needs: build  → chờ tất cả bản build xong)
   JOB "release":
        ├─ tải lại toàn bộ artifact
        ├─ tính tag phiên bản (v0.1.0-build.<số lần chạy>)
        └─ tạo GitHub Release + đính kèm 5 file binary
          │
          ▼
   ✅ Có bản Release tải về được, không cần build local
```

---

## 4. Giải thích chi tiết file `auto-release.yml`

Dưới đây là từng phần của file, kèm giải thích **từng dòng quan trọng**.

### 4.1. Tên và trigger

```yaml
name: Auto Release          # Tên hiển thị trong tab Actions

on:                         # "on" = khi nào chạy
  push:
    branches:
      - main                # Chạy mỗi khi có commit push lên nhánh main
  workflow_dispatch:        # Cho phép bấm nút "Run workflow" để chạy tay
```

- `push.branches: [main]`: chỉ nhánh `main` mới kích hoạt → tránh chạy lung tung.
- `workflow_dispatch`: thêm nút bấm chạy thủ công trên giao diện GitHub (hữu ích
  khi muốn build lại mà không cần commit mới).

### 4.2. Quyền và biến môi trường chung

```yaml
permissions:
  contents: write           # Cho GITHUB_TOKEN quyền GHI vào repo (để tạo tag + release)

env:
  CGO_ENABLED: "0"          # Build thuần Go, không cần trình biên dịch C
```

- **Vì sao cần `contents: write`?** Tạo Release = ghi dữ liệu vào repo. Mặc định
  token chỉ có quyền đọc, nên phải cấp quyền ghi, nếu không bước tạo Release sẽ lỗi `403`.
- **Vì sao `CGO_ENABLED=0`?** Tắt CGO giúp **build chéo** (cross-compile) cho
  macOS/Windows/ARM ngay trên máy Linux mà không cần cài thêm toolchain C.
  Dự án dùng SQLite thuần Go (`modernc.org/sqlite`) nên tắt CGO vẫn chạy tốt.

### 4.3. Job `build` – biên dịch đa nền tảng

```yaml
jobs:
  build:
    name: Build ${{ matrix.goos }}/${{ matrix.goarch }}
    runs-on: ubuntu-latest          # Máy ảo Linux do GitHub cấp
    strategy:
      fail-fast: false              # 1 nền tảng lỗi KHÔNG hủy các nền tảng khác
      matrix:
        include:                    # Danh sách 5 cặp (hệ điều hành, kiến trúc CPU)
          - { goos: linux,   goarch: amd64 }
          - { goos: linux,   goarch: arm64 }
          - { goos: windows, goarch: amd64 }
          - { goos: darwin,  goarch: amd64 }   # darwin = macOS
          - { goos: darwin,  goarch: arm64 }   # macOS Apple Silicon (M1/M2/M3)
```

- **`matrix`** biến 1 job thành **5 job song song**, mỗi job build cho 1 nền tảng.
  `${{ matrix.goos }}` và `${{ matrix.goarch }}` là biến thay đổi theo từng dòng.
- **`fail-fast: false`** rất quan trọng: nếu để mặc định (`true`), một bản lỗi sẽ
  hủy luôn 4 bản còn lại. Tắt đi để các bản tốt vẫn hoàn thành.

Các step trong job `build`:

```yaml
    steps:
      - name: Checkout source
        uses: actions/checkout@v4        # Tải mã nguồn về runner

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod        # Lấy đúng phiên bản Go ghi trong go.mod
          cache: true                    # Lưu cache module → lần sau nhanh hơn

      - name: Generate GUI assets
        run: go generate ./lib/api/auto  # Sinh file giao diện web nhúng vào binary
                                         # (xem Phụ lục mục 10)

      - name: Build binary
        shell: bash
        run: |
          set -euo pipefail
          bin="syncthing"
          if [ "${{ matrix.goos }}" = "windows" ]; then bin="syncthing.exe"; fi
          out="syncthing-${{ matrix.goos }}-${{ matrix.goarch }}"
          mkdir -p build dist
          GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} \
            go build -trimpath -ldflags="-s -w" -o "build/$bin" ./cmd/syncthing
          # Đóng gói: Windows -> .zip, còn lại -> .tar.gz
          if [ "${{ matrix.goos }}" = "windows" ]; then
            (cd build && zip "../dist/$out.zip" "$bin")
          else
            tar -czf "dist/$out.tar.gz" -C build "$bin"
          fi

      - name: Upload build artifact
        uses: actions/upload-artifact@v4
        with:
          name: syncthing-${{ matrix.goos }}-${{ matrix.goarch }}
          path: dist/*
          if-no-files-found: error       # Không có file = coi như lỗi
```

Giải thích các tham số build:

- `GOOS` / `GOARCH`: biến chuẩn của Go để **build chéo** (ví dụ `GOOS=windows`
  cho ra file `.exe` ngay trên máy Linux).
- `-trimpath`: bỏ đường dẫn tuyệt đối khỏi binary → gọn và “sạch” hơn.
- `-ldflags="-s -w"`: bỏ thông tin debug → **file nhỏ hơn** (~25–35 MB).
- `upload-artifact`: cất binary thành **artifact** để job `release` lấy lại được
  (vì 2 job chạy trên 2 máy ảo khác nhau, không dùng chung ổ đĩa).

### 4.4. Job `release` – tạo bản phát hành

```yaml
  release:
    name: Publish GitHub Release
    needs: build                     # Chờ TẤT CẢ bản build xong mới chạy
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: artifacts
          merge-multiple: true       # Gộp mọi artifact vào 1 thư mục phẳng

      - name: Compute release tag
        id: ver
        run: echo "tag=v0.1.0-build.${{ github.run_number }}" >> "$GITHUB_OUTPUT"

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          tag_name: ${{ steps.ver.outputs.tag }}
          name: syncthingMem0 ${{ steps.ver.outputs.tag }}
          generate_release_notes: true   # Tự sinh changelog từ commit/PR
          files: artifacts/**/*          # Đính kèm toàn bộ binary
```

Điểm cần hiểu:

- **`needs: build`**: tạo quan hệ phụ thuộc. Job `release` chỉ bắt đầu khi **toàn bộ**
  5 bản build thành công → đảm bảo Release luôn đủ 5 file.
- **`github.run_number`**: số thứ tự lần chạy (1, 2, 3, …). Dùng làm tag nên mỗi
  lần push tạo **một tag/Release mới, không trùng** (ví dụ `v0.1.0-build.1`,
  `v0.1.0-build.2`…). Đây cũng là **cách đánh phiên bản tự động**.
- **`id: ver` + `$GITHUB_OUTPUT`**: cách một step “xuất” giá trị cho step sau dùng
  lại qua cú pháp `${{ steps.ver.outputs.tag }}`.
- **`softprops/action-gh-release@v2`**: action phổ biến để tạo Release và upload
  file. Nó tự tạo tag tại đúng commit vừa push.
- **`generate_release_notes: true`**: GitHub tự viết phần “Có gì mới” dựa trên các
  commit/PR kể từ release trước.

---

## 5. Vòng đời một lần chạy thực tế

Ví dụ thật vừa chạy (lần đầu tiên):

1. `git push origin main` → GitHub nhận sự kiện `push`.
2. Workflow **Auto Release** khởi động; workflow **Mirrors** bị *skip* (do có điều
   kiện chỉ chạy cho chủ repo `syncthing`).
3. 5 job build chạy song song, mỗi job ~50–60 giây.
4. Job `release` chờ đủ 5 bản, tải artifact, tạo Release.
5. Kết quả: Release **`syncthingMem0 v0.1.0-build.1`** (gắn nhãn *Latest*) gồm:
   - `syncthing-linux-amd64.tar.gz`
   - `syncthing-linux-arm64.tar.gz`
   - `syncthing-windows-amd64.zip`
   - `syncthing-darwin-amd64.tar.gz`
   - `syncthing-darwin-arm64.tar.gz`

> Toàn bộ mất khoảng 1–2 phút, hoàn toàn trên hạ tầng GitHub.

---

## 6. Cách xem kết quả và tải bản build

**Trên giao diện web:**

- Vào repo → tab **Actions** → chọn workflow **Auto Release** để xem log từng step.
- Vào tab **Releases** (cột phải trang chủ repo) → tải file phù hợp HĐH của bạn.

**Bằng dòng lệnh (GitHub CLI `gh`):**

```bash
# Xem các lần chạy gần nhất
gh run list --limit 5

# Xem chi tiết một lần chạy
gh run view <RUN_ID>

# Theo dõi trực tiếp tới khi xong (thoát mã !=0 nếu thất bại)
gh run watch <RUN_ID> --exit-status

# Liệt kê release và tải binary
gh release list
gh release download v0.1.0-build.1            # tải toàn bộ file của release
```

---

## 7. Các workflow khác trong repo & vì sao đã “tắt”

Repo kế thừa rất nhiều workflow từ Syncthing gốc. Để nhánh `main` **chỉ chạy đúng
pipeline release**, tránh hàng loạt báo đỏ gây nhiễu, ta đã chỉnh `on:` của chúng:

| Workflow | Trước | Sau khi sửa | Lý do |
|---|---|---|---|
| `auto-release.yml` | (mới tạo) | `push: main` + thủ công | **Pipeline chính** |
| `ci.yml` | push main + PR | **chỉ PR + thủ công** | Kiểm tra format/vet/test khi mở PR, không đụng main |
| `build-syncthing.yaml` | push mọi nhánh | **chỉ thủ công** | Workflow gốc rất nặng, nhiều thứ cần secret/ký số |
| `build-client-server.yml` | push mọi nhánh | **chỉ khi publish release + thủ công** | Tránh chạy trùng với auto-release |
| `mirrors.yaml` | push | (giữ nguyên) | Tự *skip* vì có điều kiện chỉ chạy cho chủ repo gốc |

> Các workflow “tắt” **không bị xoá**. Bạn vẫn chạy tay được: tab **Actions** →
> chọn workflow → nút **Run workflow** (nhờ có `workflow_dispatch`).

---

## 8. Tùy biến pipeline theo nhu cầu

**a) Thêm/bớt nền tảng build:** sửa khối `matrix.include` trong `auto-release.yml`.
Ví dụ thêm Windows ARM64:

```yaml
- { goos: windows, goarch: arm64 }
```

**b) Đổi cách đánh phiên bản:** sửa dòng tính `tag`. Ví dụ dùng phiên bản cố định
do bạn nhập tay khi chạy `workflow_dispatch`:

```yaml
on:
  workflow_dispatch:
    inputs:
      version:
        description: "Tên phiên bản, vd v1.2.3"
        required: true
# ...
      - run: echo "tag=${{ inputs.version }}" >> "$GITHUB_OUTPUT"
```

**c) Chỉ build khi gắn tag (thay vì mỗi lần push):**

```yaml
on:
  push:
    tags:
      - "v*"        # chỉ chạy khi bạn tạo tag bắt đầu bằng v
```

Khi đó phát hành chủ động bằng:

```bash
git tag v1.0.0 && git push origin v1.0.0
```

**d) Đánh dấu bản nháp / tiền phát hành:** thêm vào step tạo release:

```yaml
          prerelease: true     # hoặc draft: true
```

---

## 9. Xử lý sự cố thường gặp

| Triệu chứng | Nguyên nhân thường gặp | Cách xử lý |
|---|---|---|
| Bước tạo Release lỗi **403** | Thiếu quyền ghi | Đảm bảo có `permissions: contents: write` |
| `auto.Assets undefined` khi build | Quên sinh GUI assets | Phải có step `go generate ./lib/api/auto` trước `go build` |
| Build chéo macOS/Windows lỗi | Bật CGO | Đặt `CGO_ENABLED: "0"` |
| Lỗi định dạng ở CI (gofmt) | File để CRLF/định dạng sai | Chạy `gofmt -w <file>` rồi commit lại |
| Release bị **trùng tag** | Dùng tag cố định, push lại | Dùng `github.run_number` (đã áp dụng) hoặc xoá tag cũ |
| Cảnh báo **Node.js 20 deprecated** | Action đời cũ | *Chỉ là cảnh báo, không lỗi.* Khi cần, nâng lên `actions/checkout@v5`, `actions/setup-go@v6` |

**Cách đọc log lỗi nhanh bằng CLI:**

```bash
gh run view <RUN_ID> --log-failed     # chỉ in log các step thất bại
```

---

## 10. Phụ lục: vì sao phải “generate assets”

Syncthing nhúng **giao diện web (GUI)** trực tiếp vào file binary. Phần GUI này
được sinh ra thành file Go `lib/api/auto/gui.files.go` từ thư mục `gui/`.

- File `gui.files.go` **không được commit** (nằm trong `.gitignore`) vì nó rất lớn
  (~6 MB) và là sản phẩm sinh tự động.
- Khi không có file này, gói `lib/api` thiếu hàm `auto.Assets()` → **build sẽ lỗi**.
- Vì vậy mọi pipeline build **bắt buộc** chạy `go generate ./lib/api/auto` trước khi
  `go build`. Lệnh này gọi `script/genassets.go` đọc thư mục `gui/` và sinh file.

> Trước đây hàm `rebuildAssets()` trong `build.go` bị để rỗng (không sinh gì) và
> `lazyRebuildAssets()` bị lỗi cú pháp, khiến mọi build phụ thuộc `go run build.go`
> đều hỏng. Hai lỗi này đã được sửa, nên giờ cả `go run build.go` lẫn
> `go generate ./lib/api/auto` đều sinh assets đúng.

---

### Tóm tắt một câu

> **Bạn chỉ cần `git push` lên `main`. GitHub sẽ tự build 5 nền tảng và tạo một bản
> Release tải về được — không cần build trên máy cá nhân nữa.**
