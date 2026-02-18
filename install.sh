#!/usr/bin/env bash
set -euo pipefail

REPO="${CONSULT_HUMAN_REPO:-AlhasanIQ/consult-human}"
VERSION="${CONSULT_HUMAN_VERSION:-latest}"
INSTALL_DIR="${CONSULT_HUMAN_INSTALL_DIR:-$HOME/.local/bin}"
SETUP_MODE="${CONSULT_HUMAN_SETUP_MODE:-auto}"
SETUP_PROVIDER="${CONSULT_HUMAN_SETUP_PROVIDER:-}"

log() {
  printf '[consult-human installer] %s\n' "$*"
}

die() {
  printf '[consult-human installer] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Install consult-human from GitHub Releases.

Usage:
  install.sh [options]

Options:
  --version <tag|latest>            Version tag to install (default: latest)
  --install-dir <path>              Binary install directory (default: ~/.local/bin)
  --repo <owner/repo>               GitHub repo (default: AlhasanIQ/consult-human)
  --setup-mode <auto|interactive|non-interactive|skip>
                                    setup mode after install (default: auto)
  --non-interactive-setup           Shortcut for --setup-mode non-interactive
  --provider <telegram>             Optional provider flag passed to setup
  --help                            Show this help

Environment overrides:
  CONSULT_HUMAN_REPO
  CONSULT_HUMAN_VERSION
  CONSULT_HUMAN_INSTALL_DIR
  CONSULT_HUMAN_SETUP_MODE
  CONSULT_HUMAN_SETUP_PROVIDER
EOF
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || die "missing required command: $cmd"
}

download_to_file() {
  local url="$1"
  local dst="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dst"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$dst" "$url"
    return
  fi
  die "curl or wget is required"
}

download_to_stdout() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO- "$url"
    return
  fi
  die "curl or wget is required"
}

detect_os() {
  local u
  u="$(uname -s)"
  case "$u" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *) die "unsupported OS: $u (supported: linux, darwin)" ;;
  esac
}

detect_arch() {
  local u
  u="$(uname -m)"
  case "$u" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) die "unsupported architecture: $u (supported: amd64, arm64)" ;;
  esac
}

resolve_version() {
  if [[ "$VERSION" != "latest" ]]; then
    echo "$VERSION"
    return
  fi

  local api_url json tag
  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  json="$(download_to_stdout "$api_url")"
  tag="$(printf '%s\n' "$json" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
  [[ -n "$tag" ]] || die "could not resolve latest release tag from ${api_url}"
  echo "$tag"
}

sha256_file() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  die "sha256sum or shasum is required for checksum verification"
}

verify_checksum() {
  local archive="$1"
  local checksums="$2"
  local archive_name expected actual
  archive_name="$(basename "$archive")"
  expected="$(grep -E "([[:space:]]|\\*)${archive_name}\$" "$checksums" | awk '{print $1}' | tail -n1)"
  [[ -n "$expected" ]] || die "checksum for ${archive_name} not found in checksums.txt"
  actual="$(sha256_file "$archive")"
  [[ "$actual" == "$expected" ]] || die "checksum mismatch for ${archive_name}"
}

install_binary() {
  local archive="$1"
  local install_dir="$2"
  local tmp_extract bin_src bin_dst

  tmp_extract="$(mktemp -d)"
  trap 'rm -rf "$tmp_extract"' RETURN
  tar -xzf "$archive" -C "$tmp_extract"

  bin_src="${tmp_extract}/consult-human"
  [[ -f "$bin_src" ]] || die "archive does not contain consult-human binary"

  mkdir -p "$install_dir"
  bin_dst="${install_dir}/consult-human"
  if command -v install >/dev/null 2>&1; then
    install -m 0755 "$bin_src" "$bin_dst"
  else
    cp "$bin_src" "$bin_dst"
    chmod 0755 "$bin_dst"
  fi
  printf '%s\n' "$bin_dst"
}

resolve_setup_mode() {
  local mode="$1"
  case "$mode" in
    auto)
      if [[ -r /dev/tty && -w /dev/tty ]]; then
        echo "interactive"
      else
        echo "non-interactive"
      fi
      ;;
    interactive|non-interactive|skip)
      echo "$mode"
      ;;
    *)
      die "invalid setup mode: $mode (expected auto|interactive|non-interactive|skip)"
      ;;
  esac
}

run_setup() {
  local bin_path="$1"
  local mode="$2"
  local provider="$3"
  local cmd=("$bin_path" "setup")

  if [[ -n "$provider" ]]; then
    cmd+=("--provider" "$provider")
  fi

  case "$mode" in
    skip)
      log "Skipping setup (--setup-mode skip)."
      return
      ;;
    non-interactive)
      log "Running setup in non-interactive mode."
      cmd+=("--non-interactive")
      "${cmd[@]}"
      return
      ;;
    interactive)
      log "Running setup in interactive mode."
      if [[ ! -r /dev/tty || ! -w /dev/tty ]]; then
        die "interactive setup requested, but /dev/tty is unavailable"
      fi
      "${cmd[@]}" </dev/tty >/dev/tty
      return
      ;;
    *)
      die "unexpected setup mode: $mode"
      ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || die "missing value for --version"
      VERSION="$2"
      shift 2
      ;;
    --install-dir)
      [[ $# -ge 2 ]] || die "missing value for --install-dir"
      INSTALL_DIR="$2"
      shift 2
      ;;
    --repo)
      [[ $# -ge 2 ]] || die "missing value for --repo"
      REPO="$2"
      shift 2
      ;;
    --setup-mode)
      [[ $# -ge 2 ]] || die "missing value for --setup-mode"
      SETUP_MODE="$2"
      shift 2
      ;;
    --non-interactive-setup)
      SETUP_MODE="non-interactive"
      shift
      ;;
    --provider)
      [[ $# -ge 2 ]] || die "missing value for --provider"
      SETUP_PROVIDER="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1 (run with --help)"
      ;;
  esac
done

require_cmd tar
require_cmd uname

OS="$(detect_os)"
ARCH="$(detect_arch)"
TAG="$(resolve_version)"
VERSION_NO_V="${TAG#v}"
ASSET="consult-human_${VERSION_NO_V}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT
ARCHIVE_PATH="${WORK_DIR}/${ASSET}"
CHECKSUMS_PATH="${WORK_DIR}/checksums.txt"

log "Downloading ${ASSET} from ${REPO} ${TAG}..."
download_to_file "${BASE_URL}/${ASSET}" "$ARCHIVE_PATH"
download_to_file "${BASE_URL}/checksums.txt" "$CHECKSUMS_PATH"

log "Verifying checksum..."
verify_checksum "$ARCHIVE_PATH" "$CHECKSUMS_PATH"

log "Installing consult-human to ${INSTALL_DIR}..."
BIN_PATH="$(install_binary "$ARCHIVE_PATH" "$INSTALL_DIR")"
log "Installed: ${BIN_PATH}"

RESOLVED_SETUP_MODE="$(resolve_setup_mode "$SETUP_MODE")"
run_setup "$BIN_PATH" "$RESOLVED_SETUP_MODE" "$SETUP_PROVIDER"

log "Done."
