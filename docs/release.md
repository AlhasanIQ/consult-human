# Release and Distribution

## CI

`CI` workflow runs on push/PR:

- `go test ./...`
- `go mod verify`
- `go vet ./...`
- reproducible build check
- installer shell syntax check (`bash -n install.sh`)

## Release

`Release` workflow runs on version tags (`v*`):

1. re-runs tests/vet/verification
2. builds tarballs for:
   - `linux/amd64`
   - `linux/arm64`
   - `darwin/amd64`
   - `darwin/arm64`
3. creates `checksums.txt`
4. publishes/updates GitHub Release assets via `gh`

## Installer Script

`install.sh`:

- detects OS/arch
- downloads release artifact + checksums
- verifies SHA256
- installs binary (default `~/.local/bin`)
- optionally runs `consult-human setup` (interactive, non-interactive, or skip)

Main options:

```bash
--version <tag|latest>
--install-dir <path>
--setup-mode <auto|interactive|non-interactive|skip>
--provider telegram
```
