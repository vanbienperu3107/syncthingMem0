# syncthingMem0

Repository cho bản Syncthing/Mem0.

## CI/CD

Repo đã có quy trình GitHub Actions để kiểm tra, build thử và phát hành theo tag.

- CI: `.github/workflows/ci.yml`
- Release: `.github/workflows/release.yml`
- Tài liệu vận hành: `docs/ci-cd.md`

Tạo release bằng tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Workflow release sẽ build artifact cho Linux, macOS và Windows, sau đó tạo GitHub Release kèm checksum.
