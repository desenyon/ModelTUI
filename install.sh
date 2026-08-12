#!/usr/bin/env bash
# ModelTUI one-line installer — downloads the latest release and puts it on your PATH.
set -euo pipefail

REPO="${MODELTUI_REPO:-desenyon/ModelTUI}"
BIN_NAME="modeltui"
INSTALL_DIR="${MODELTUI_INSTALL_DIR:-$HOME/.local/bin}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "error: need '$1' on PATH" >&2
    exit 1
  }
}

need curl
need tar
need uname

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "error: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac
case "$os" in
  darwin|linux) ;;
  *)
    echo "error: unsupported OS: $os" >&2
    exit 1
    ;;
esac

echo "==> Resolving latest release from GitHub (${REPO})..."
api="https://api.github.com/repos/${REPO}/releases/latest"
tag="$(curl -fsSL "$api" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
if [[ -z "${tag}" ]]; then
  echo "error: could not determine latest tag" >&2
  exit 1
fi
version="${tag#v}"
asset="modeltui_${version}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

echo "==> Downloading ${asset}..."
curl -fsSL "${url}" -o "${tmpdir}/${asset}"
tar -xzf "${tmpdir}/${asset}" -C "${tmpdir}"

if [[ ! -f "${tmpdir}/${BIN_NAME}" ]]; then
  echo "error: archive did not contain ${BIN_NAME}" >&2
  exit 1
fi

mkdir -p "${INSTALL_DIR}"
install -m 755 "${tmpdir}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"

# Ensure INSTALL_DIR is on PATH for future shells.
path_line="export PATH=\"${INSTALL_DIR}:\$PATH\""
append_path() {
  local rc="$1"
  [[ -f "${rc}" ]] || touch "${rc}"
  if grep -Fqs "${INSTALL_DIR}" "${rc}"; then
    return 0
  fi
  {
    echo ""
    echo "# ModelTUI"
    echo "${path_line}"
  } >>"${rc}"
  echo "==> Added ${INSTALL_DIR} to PATH in ${rc}"
}

case "${SHELL##*/}" in
  zsh) append_path "${HOME}/.zshrc" ;;
  bash)
    if [[ -f "${HOME}/.bashrc" ]]; then
      append_path "${HOME}/.bashrc"
    else
      append_path "${HOME}/.bash_profile"
    fi
    ;;
  fish)
    fish_cfg="${HOME}/.config/fish/config.fish"
    mkdir -p "$(dirname "${fish_cfg}")"
    if ! grep -Fqs "${INSTALL_DIR}" "${fish_cfg}" 2>/dev/null; then
      {
        echo ""
        echo "# ModelTUI"
        echo "fish_add_path ${INSTALL_DIR}"
      } >>"${fish_cfg}"
      echo "==> Added ${INSTALL_DIR} to PATH in ${fish_cfg}"
    fi
    ;;
  *)
    append_path "${HOME}/.profile"
    ;;
esac

export PATH="${INSTALL_DIR}:${PATH}"

echo ""
echo "Installed ${BIN_NAME} ${tag} -> ${INSTALL_DIR}/${BIN_NAME}"
echo ""
echo "Start it with:"
echo "  modeltui"
echo ""
if ! command -v modeltui >/dev/null 2>&1; then
  echo "If modeltui is not found yet, run:"
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
  echo "  # or open a new terminal"
fi
