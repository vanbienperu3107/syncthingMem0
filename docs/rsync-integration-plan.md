# Kế hoạch tích hợp rsync (delta transfer) vào luồng sync

Tài liệu này mô tả cách nối bộ máy delta `lib/rsync` vào luồng truyền block
của Syncthing, phần còn lại sau khi đã hoàn thành nền tảng ở `lib/rsync`
(streaming `Deltify` + API per-block `SignatureBytes`/`DeltaBytes`/`ApplyDelta`
trong `lib/rsync/wire.go`).

Mục tiêu: giảm băng thông khi một block đã thay đổi nhưng phía nhận còn giữ
phiên bản cũ của đúng block đó — chỉ truyền phần khác nhau bên trong block.

## 1. Bối cảnh: Syncthing đã có gì

Syncthing vốn là hệ delta-transfer **theo block content-addressed**:

- File chia thành block (128 KiB–16 MiB), mỗi block có strong hash SHA-256
  (`proto/bep/bep.proto`, `BlockInfo`).
- Khi pull (`lib/model/folder_sendrecv.go`), `copierRoutine` tái dùng mọi block
  trùng hash trên đĩa (temp file, file hiện tại, mọi folder có block index).
  Chỉ block **không** tìm được bản sao cục bộ mới đi qua mạng
  (`pullBlock` → `RequestGlobal`).

Vì vậy rsync **không** thay thế cơ chế này; nó chỉ bổ sung **delta trong phạm vi
một block** cho trường hợp: block đã đổi (hash khác) nhưng phía nhận có phiên bản
cũ của cùng block tại cùng offset, và phần đổi là cục bộ trong block.

## 2. Điểm cắm: sub-block delta (per-block)

Cắm vào đúng đường `pullBlock` (`lib/model/folder_sendrecv.go`), **sau** khi
`copierRoutine` đã loại hết block copy được cục bộ:

```
Phía NHẬN (requester) — có block cũ O tại offset, cần block mới N:
  1. sig  = rsync.SignatureBytes(O)          # O là block cũ, <=16MB, nằm RAM
  2. gửi RequestDelta{folder, name, offset, size, baseSignature=sig}

Phía GỬI (responder) — có block mới N:
  3. N   = đọc block tại offset (readOffsetIntoBuf như hiện tại)
  4. delta = rsync.DeltaBytes(N, sig)
  5. trả ResponseDelta{data=delta}

Phía NHẬN:
  6. N2 = rsync.ApplyDelta(O, delta)          # dựng lại block mới
  7. verify SHA-256(N2) == block.Hash          # BẮT BUỘC, như pullBlock hiện tại
  8. WriteAt(N2, offset)                        # giữ ghi song song/sparse
```

Vì sao cách này an toàn về kiến trúc:

- **Không OOM**: base là *một block* (≤16 MiB) trong RAM, không phải cả file.
  `Deltify` đã streaming nên không giữ thêm bản sao.
- **Giữ pipeline song song + sparse**: mỗi block xử lý độc lập, kết quả `WriteAt`
  vào đúng offset — không đụng mô hình ghi tuần tự.
- **Không giảm đảm bảo toàn vẹn**: sau khi `ApplyDelta`, vẫn verify SHA-256 của
  block như đường pull hiện tại. Nếu delta sai → hash lệch → fallback tải block
  đầy đủ. rsync dùng SHA-1 nội bộ chỉ để *tìm* khối; tính đúng đắn cuối cùng do
  SHA-256 của Syncthing bảo chứng.

## 3. Thay đổi ở tầng protocol (BEP)

Cần công cụ regenerate protobuf (`buf` + plugin buf.build).

1. `proto/bep/bep.proto`:
   - Thêm `MessageType`: `MESSAGE_TYPE_REQUEST_DELTA`, `MESSAGE_TYPE_RESPONSE_DELTA`
     (hoặc thêm field vào `Request`/`Response` — xem mục 6).
   - `RequestDelta` = `Request` + `bytes base_signature`.
   - `ResponseDelta` = `Response` (field `data` mang delta blob thay vì block thô).
2. `internal/gen/bep`: regenerate (`buf generate`, xem `buf.gen.yaml`).
3. `lib/protocol/protocol.go`: thêm dispatch cho message mới trong vòng đọc
   `rawConnection`, và API `RequestDelta(...)` song song với `Request(...)`.
4. `lib/protocol/bep_request_response.go`: wrapper toWire/fromWire cho message mới.

## 4. Thương lượng khả năng (capability negotiation) — BẮT BUỘC

Peer cũ không hiểu message delta. Phải fallback an toàn:

- Thêm cờ vào `Hello` hoặc `ClusterConfig` (ví dụ `bool supports_block_delta`).
- Requester chỉ gửi `RequestDelta` khi peer quảng bá hỗ trợ **và** có base O.
- Nếu không, dùng `Request` block thô như cũ.
- Nếu `RequestDelta` lỗi ở bất kỳ bước nào (hash lệch, decode lỗi) → retry bằng
  `Request` block đầy đủ. Delta là **tối ưu hoá tùy chọn**, không phải đường
  bắt buộc.

## 5. Thay đổi ở tầng model

`lib/model/folder_sendrecv.go`:

- `pullBlock`: nếu bật cờ folder + peer hỗ trợ + có `curFile`/temp chứa block cũ
  tại offset → đọc O, `SignatureBytes(O)`, gọi `RequestDelta`, `ApplyDelta`,
  verify hash, `WriteAt`. Ngược lại giữ nguyên `pullBlock` hiện tại.
- `model.Request` phía responder: thêm handler cho `RequestDelta` — đọc block N
  như `readOffsetIntoBuf`, `DeltaBytes(N, baseSignature)`, trả delta.

Thêm cờ bật/tắt theo folder (mặc định TẮT), ví dụ `FolderConfiguration.BlockDelta`,
tương tự `IncrementalScan`/`UseLWWReconciler`.

## 6. Phương án thay thế gọn hơn (khuyến nghị cân nhắc)

Thay vì thêm 2 message type, có thể **mở rộng `Request`/`Response` hiện có**:

- `Request` thêm `bytes base_signature` (optional). Rỗng = request block thô.
- `Response`: nếu request có signature, `data` chứa delta blob (một flag/`code`
  báo hiệu là delta). Rỗng = block thô.

Ưu điểm: ít thay đổi dispatch, dễ negotiation (chỉ cần 1 cờ capability để biết
peer có đọc được `base_signature` không). Nhược điểm: ngữ nghĩa field `data` phụ
thuộc ngữ cảnh.

## 7. Kiểm thử BẮT BUỘC trước khi bật mặc định

An toàn dữ liệu là mục tiêu số 1. Không bật `BlockDelta` mặc định cho tới khi:

1. Unit test model cho cả hai nhánh (delta thành công, delta lỗi → fallback).
2. **Test sync đa-thiết-bị thật**: 2+ instance, sửa file cục bộ nhiều vòng, kiểm
   tra nội dung khớp bit-for-bit hai đầu, có ngắt kết nối giữa chừng.
3. Test tương thích ngược: một peer bật, một peer không hỗ trợ → vẫn sync đúng.
4. Fuzz `ApplyDelta` với delta hỏng (đã có test từ chối ở `wire_test.go`, mở
   rộng thêm ở tầng model).

## 8. Trạng thái hiện tại

| Phần | Trạng thái |
|---|---|
| `Deltify` streaming (không OOM) | ✅ xong (`lib/rsync/engine.go`) |
| API per-block + wire codec | ✅ xong (`lib/rsync/wire.go`, có test) |
| Message BEP + regenerate proto | ⬜ chờ (cần `buf` + mạng buf.build) |
| Capability negotiation | ⬜ chờ |
| Model `pullBlock`/`Request` | ⬜ chờ |
| Cờ folder `BlockDelta` (mặc định tắt) | ⬜ chờ |
| Test đa thiết bị | ⬜ chờ (cần môi trường chạy 2+ instance) |

Môi trường cần để hoàn tất: có `buf` + truy cập buf.build (regenerate proto) và
khả năng chạy nhiều instance để test sync live.
