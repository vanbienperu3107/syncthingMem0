# CI/CD cho syncthingMem0

Tài liệu này mô tả quy trình build, test và release cho repo `vanbienperu3107/syncthingMem0`.

## Mục tiêu

- Pull request luôn được kiểm tra trước khi merge.
- Nhánh `main` luôn ở trạng thái có thể release.
- Release được tạo bằng Git tag, không tạo thủ công trên máy cá nhân.
- Artifact release gồm binary theo hệ điều hành/kiến trúc và checksum SHA256.

## Nhánh và tag

- `main`: nhánh ổn định, chỉ merge khi CI xanh.
- `feature/*`: nhánh phát triển tính năng.
- `fix/*`: nhánh sửa lỗi.
- `release/*`: tùy chọn, dùng khi cần gom thay đổi trước bản phát hành lớn.
- `vX.Y.Z`: tag release chính thức, ví dụ `v1.2.0`.
- `vX.Y.Z-rc.N`: tag thử nghiệm trước release, ví dụ `v1.2.0-rc.1`.

## Quy trình build và test

1. Tạo nhánh mới từ `main`.
2. Commit thay đổi và mở pull request.
3. GitHub Actions chạy:
   - kiểm tra định dạng Go bằng `gofmt`;
   - chạy `go vet`;
   - chạy unit test bằng `go test ./...`;
   - build thử binary cho Linux, Windows và macOS.
4. Chỉ merge khi tất cả job thành công.

Workflow CI hiện tự phát hiện các thư mục có `go.mod`. Nếu repo chưa có mã nguồn Go, CI sẽ bỏ qua phần Go và báo thành công để repo vẫn dùng được trong giai đoạn khởi tạo.

## Quy trình release

1. Đảm bảo `main` đã xanh trên CI.
2. Tạo tag theo Semantic Versioning:

   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. GitHub Actions tự động:
   - test lại trước khi release;
   - build binary theo ma trận OS/architecture;
   - nén artifact;
   - tạo file checksum;
   - tạo GitHub Release và đính kèm artifact.

## Artifact release

Tên artifact có dạng:

```text
syncthingMem0_<version>_<goos>_<goarch>.tar.gz
syncthingMem0_<version>_<goos>_<goarch>.zip
checksums.txt
```

Windows dùng `.zip`, các nền tảng còn lại dùng `.tar.gz`.

## Điều kiện để release thành công

Repo cần có mã nguồn Go và ít nhất một entrypoint chính:

```text
cmd/syncthing/main.go
```

Nếu dự án đổi tên binary hoặc entrypoint, cập nhật biến `BINARY_NAME` và `MAIN_PACKAGE` trong `.github/workflows/release.yml`.

## Quy tắc bảo vệ đề xuất

Trong GitHub, bật branch protection cho `main`:

- Require a pull request before merging.
- Require status checks to pass before merging.
- Require branches to be up to date before merging.
- Restrict direct pushes to `main`.

## Rollback

Nếu release lỗi:

1. Gỡ release lỗi trên GitHub nếu artifact không dùng được.
2. Tạo tag mới với patch version tăng lên, ví dụ từ `v1.0.0` sang `v1.0.1`.
3. Không ghi đè tag đã public trừ khi chắc chắn chưa ai dùng tag đó.
