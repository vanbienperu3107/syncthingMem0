# Auto deploy cho optional server

Repo nay khong bat buoc phai co backend server de chay phan mem dong bo. Tuy
nhien, neu sau nay ban dua source `cmd/strelaysrv` va hoac `cmd/stdiscosrv`
vao repo, workflow nay se tu build image va deploy len VPS.

## Pham vi

- `strelaysrv`: relay server cho truong hop client khong ket noi truc tiep duoc.
- `stdiscosrv`: discovery server neu muon tu van hanh discovery.

Neu repo chua co `go.mod` o root hoac chua co hai thu muc tren, workflow deploy
se tu bo qua.

## Tep da tao

- `.github/workflows/deploy-optional-servers.yml`
- `deploy/Dockerfile.server`
- `deploy/docker-compose.optional-servers.yml`

## Cach kich hoat

Workflow se chay trong 2 truong hop:

1. Tu dong khi push tag `v*`, vi du `v1.0.0`
2. Chay tay bang `workflow_dispatch`

## Secrets can cau hinh tren GitHub

Bat buoc:

- `DEPLOY_HOST`: IP hoac domain cua VPS
- `DEPLOY_USER`: user SSH tren VPS
- `DEPLOY_SSH_KEY`: private key dung de SSH

Tuy chon:

- `DEPLOY_PORT`: mac dinh `22`
- `DEPLOY_PATH`: mac dinh `/opt/syncthingmem0`
- `GHCR_USERNAME`: can neu package khong public
- `GHCR_TOKEN`: can neu package khong public
- `RELAY_EXT_ADDRESS`: dia chi relay quang ba ra ngoai, vi du `203.0.113.10:22067`
- `RELAY_POOLS`: de trong de relay private, hoac dat pool can tham gia
- `RELAY_TOKEN`: token gioi han truy cap relay private
- `RELAY_PROVIDED_BY`: chuoi mo ta nha van hanh relay
- `DISCOVERY_HTTP`: dat `true` neu chay sau HTTPS proxy
- `DISCOVERY_METRICS_LISTEN`: vi du `:9090`

## Yeu cau tren VPS

- Da cai `docker`
- Da cai `docker compose`
- Mo port can thiet:
  - Relay: `22067/tcp`, tuy chon them `22070/tcp` cho status
  - Discovery: `8443/tcp`

## Quy trinh deploy

1. GitHub Actions build image tu source server co san trong repo
2. Push image len GHCR
3. Copy `docker-compose.optional-servers.yml` len VPS
4. Tao file `.env` tren VPS
5. Chay `docker compose pull`
6. Chay `docker compose up -d --remove-orphans`

## Cach dung

Release auto deploy:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Deploy tay:

- Mo workflow `Deploy Optional Servers`
- Chon `services=auto` de tu nhan dien
- Hoac chon `strelaysrv` / `stdiscosrv` de ep deploy mot service

## Luu y

- Workflow nay duoc thiet ke an toan cho giai doan khoi tao repo: khong co
  source server thi no skip, khong fail vo nghia.
- `strelaysrv` va `stdiscosrv` khong phai thanh phan bat buoc cho moi cai dat
  Syncthing. Chi nen deploy neu ban muon tu quan ly ha tang relay/discovery.
