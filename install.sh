#!/usr/bin/env sh
set -eu

REPO_OWNER="AppajiDheeraj"
REPO_NAME="GoFetch"
BINARY_NAME="gofetch"
INSTALL_DIR_DEFAULT="/usr/local/bin"
INSTALL_DIR_FALLBACK="$HOME/.local/bin"

say() {
  printf '%s\n' "$*"
}

fail() {
  say "ERROR: $*"
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

arch_normalize() {
  case "$1" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) echo "" ;;
  esac
}

os_normalize() {
  case "$1" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *) echo "" ;;
  esac
}

say "Installing GoFetch..."

OS="$(uname -s)"
ARCH="$(uname -m)"
OS="$(os_normalize "$OS")"
ARCH="$(arch_normalize "$ARCH")"

[ -n "$OS" ] || fail "Unsupported OS"
[ -n "$ARCH" ] || fail "Unsupported architecture"

VERSION="${GOFETCH_VERSION:-latest}"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/latest/download/${BINARY_NAME}-${OS}-${ARCH}"
else
  URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/${VERSION}/${BINARY_NAME}-${OS}-${ARCH}"
fi

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t gofetch)"
BIN_PATH="$TMP_DIR/$BINARY_NAME"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

if need_cmd curl; then
  curl -fsSL "$URL" -o "$BIN_PATH"
elif need_cmd wget; then
  wget -qO "$BIN_PATH" "$URL"
else
  fail "curl or wget is required"
fi

chmod +x "$BIN_PATH"

INSTALL_DIR="$INSTALL_DIR_DEFAULT"
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="$INSTALL_DIR_FALLBACK"
  mkdir -p "$INSTALL_DIR"
fi

if [ -w "$INSTALL_DIR_DEFAULT" ]; then
  mv "$BIN_PATH" "$INSTALL_DIR_DEFAULT/$BINARY_NAME"
else
  mv "$BIN_PATH" "$INSTALL_DIR/$BINARY_NAME"
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) : ;;
    *)
      SHELL_NAME="$(basename "${SHELL:-sh}")"
      PROFILE=""
      if [ "$SHELL_NAME" = "bash" ]; then
        PROFILE="$HOME/.bashrc"
      elif [ "$SHELL_NAME" = "zsh" ]; then
        PROFILE="$HOME/.zshrc"
      elif [ "$SHELL_NAME" = "fish" ]; then
        PROFILE="$HOME/.config/fish/config.fish"
      fi
      if [ -n "$PROFILE" ]; then
        say "Adding $INSTALL_DIR to PATH in $PROFILE"
        if [ "$SHELL_NAME" = "fish" ]; then
          printf '\nset -Ux PATH %s $PATH\n' "$INSTALL_DIR" >> "$PROFILE"
        else
          printf '\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$PROFILE"
        fi
      else
        say "Add $INSTALL_DIR to your PATH"
      fi
      ;;
  esac
fi

say "GoFetch installed successfully. Run: gofetch --help"
