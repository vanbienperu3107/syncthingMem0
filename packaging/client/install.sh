#!/usr/bin/env sh
# syncthingMem0 — CLIENT installer (Linux / macOS)
#
# Cài binary + TỰ TẠO CẤU HÌNH cho máy client kết nối tới hub qua WSS:443.
# Cấu hình lấy từ biến môi trường, nếu thiếu sẽ hỏi tương tác:
#   HUB_URL        ví dụ: vps.example.com:443   (bắt buộc)
#   HUB_DEVICE_ID  device ID của hub             (bắt buộc để ghép cặp)
#   HUB_TOKEN      JWT do hub cấp (/api/register)(tùy chọn, lưu vào config)
#   FOLDER_PATH    thư mục đồng bộ   (mặc định: $HOME/SyncMem0)
#   FOLDER_ID      id thư mục        (mặc định: default)
#   BIN_DIR        nơi cài binary    (mặc định: $HOME/.local/bin)
#   STHOMEDIR      thư mục config    (mặc định: $HOME/.config/syncthingmem0)
#
# Dùng: ./install.sh   (chạy trong thư mục đã giải nén)
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BIN_SRC="$SCRIPT_DIR/syncthing"
[ -f "$BIN_SRC" ] || { echo "Không tìm thấy binary 'syncthing' cạnh script."; exit 1; }

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
CFG_DIR="${STHOMEDIR:-$HOME/.config/syncthingmem0}"
HUB_URL="${HUB_URL:-}"
HUB_DEVICE_ID="${HUB_DEVICE_ID:-}"
HUB_TOKEN="${HUB_TOKEN:-}"
FOLDER_PATH="${FOLDER_PATH:-$HOME/SyncMem0}"
FOLDER_ID="${FOLDER_ID:-default}"

prompt() { # $1=var name, $2=question
  eval "v=\${$1}"
  if [ -z "$v" ] && [ -t 0 ]; then
    printf "%s: " "$2"; read -r v; eval "$1=\$v"
  fi
}
prompt HUB_URL       "Địa chỉ hub (host:443)"
prompt HUB_DEVICE_ID "Device ID của hub"
prompt HUB_TOKEN     "Hub token (Enter để bỏ qua)"

echo ">> Cài binary vào $BIN_DIR"
mkdir -p "$BIN_DIR" "$CFG_DIR" "$FOLDER_PATH"
cp "$BIN_SRC" "$BIN_DIR/syncthingmem0"
chmod +x "$BIN_DIR/syncthingmem0"

echo ">> Tạo cấu hình + khóa thiết bị tại $CFG_DIR"
STHOMEDIR="$CFG_DIR" "$BIN_DIR/syncthingmem0" generate --no-port-probing

CONF="$CFG_DIR/config.xml"
SELF_ID=$(STHOMEDIR="$CFG_DIR" "$BIN_DIR/syncthingmem0" device-id 2>/dev/null || true)
[ -n "$SELF_ID" ] || SELF_ID=$(sed -n 's/.*<device id="\([A-Z0-9-]*\)".*/\1/p' "$CONF" | head -1)

# Ghép cặp hub: chèn <device hub> + <folder> + <deviceToken> vào config.xml
if [ -n "$HUB_URL" ] && [ -n "$HUB_DEVICE_ID" ]; then
  echo ">> Cấu hình kết nối tới hub $HUB_URL ($HUB_DEVICE_ID)"
  TOKEN_LINE=""
  [ -n "$HUB_TOKEN" ] && TOKEN_LINE="    <deviceToken>$HUB_TOKEN</deviceToken>"
  TMP="$CONF.tmp"
  awk -v hub="$HUB_DEVICE_ID" -v url="$HUB_URL" -v self="$SELF_ID" \
      -v fid="$FOLDER_ID" -v fpath="$FOLDER_PATH" -v tok="$TOKEN_LINE" '
    /<\/configuration>/ {
      print "    <device id=\"" hub "\" name=\"hub\" compression=\"metadata\">"
      print "        <address>wss://" url "</address>"
      print "        <paused>false</paused>"
      print "    </device>"
      print "    <folder id=\"" fid "\" label=\"SyncMem0\" path=\"" fpath "\" type=\"sendreceive\">"
      print "        <device id=\"" self "\"></device>"
      print "        <device id=\"" hub "\"></device>"
      print "    </folder>"
      if (tok != "") print tok
    }
    { print }
  ' "$CONF" > "$TMP" && mv "$TMP" "$CONF"
fi

cat <<EOF

==================================================================
 Cài đặt xong.
 Binary : $BIN_DIR/syncthingmem0
 Config : $CFG_DIR
 Thư mục: $FOLDER_PATH
 Device ID của máy này:
   $SELF_ID
 -> Hãy authorize device ID này trên hub (thêm vào folder tương ứng).

 Chạy client:
   STHOMEDIR="$CFG_DIR" "$BIN_DIR/syncthingmem0" serve
==================================================================
EOF
