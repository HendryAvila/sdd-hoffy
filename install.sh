#!/usr/bin/env bash
# SDD-Hoffy Installer
# One-liner: curl -sSL https://raw.githubusercontent.com/HendryAvila/sdd-hoffy/main/install.sh | bash
#
# This script:
#   1. Detects your OS and architecture
#   2. Downloads the latest sdd-hoffy binary from GitHub
#   3. Installs it to your PATH
#
# Works on: macOS (Intel/Apple Silicon), Linux (x86_64/arm64), WSL

set -euo pipefail

# --- Colors and formatting ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m' # No Color

# --- Helper functions ---

info() {
    printf "${BLUE}ℹ${NC}  %s\n" "$1"
}

success() {
    printf "${GREEN}✅${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}⚠️${NC}  %s\n" "$1"
}

error() {
    printf "${RED}❌${NC} %s\n" "$1" >&2
}

step() {
    printf "\n${BOLD}${CYAN}▸ %s${NC}\n" "$1"
}

# --- Banner ---

print_banner() {
    printf "\n"
    printf "${BOLD}${CYAN}"
    printf "  ╔═══════════════════════════════════════════╗\n"
    printf "  ║                                           ║\n"
    printf "  ║   🏗️  SDD-Hoffy Installer                 ║\n"
    printf "  ║   Spec-Driven Development MCP Server      ║\n"
    printf "  ║                                           ║\n"
    printf "  ╚═══════════════════════════════════════════╝\n"
    printf "${NC}\n"
    printf "  ${DIM}Think first, code second. Reduce AI hallucinations${NC}\n"
    printf "  ${DIM}by writing clear specs BEFORE generating code.${NC}\n\n"
}

# --- OS/Arch detection ---

detect_os() {
    local os
    os="$(uname -s)"

    case "$os" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*)
            error "Native Windows is not supported. Please use WSL (Windows Subsystem for Linux)."
            error "Install WSL: https://learn.microsoft.com/en-us/windows/wsl/install"
            exit 1
            ;;
        *)
            error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
}

detect_arch() {
    local arch
    arch="$(uname -m)"

    case "$arch" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)
            error "Unsupported architecture: $arch"
            error "SDD-Hoffy supports x86_64 (amd64) and arm64 only."
            exit 1
            ;;
    esac
}

# --- Version fetching ---

get_latest_version() {
    local version

    if command -v curl &>/dev/null; then
        version=$(curl -sSL "https://api.github.com/repos/HendryAvila/sdd-hoffy/releases/latest" \
            -H "Accept: application/vnd.github.v3+json" \
            | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/')
    elif command -v wget &>/dev/null; then
        version=$(wget -qO- "https://api.github.com/repos/HendryAvila/sdd-hoffy/releases/latest" \
            | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/')
    else
        error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi

    if [ -z "$version" ]; then
        error "Could not determine the latest version."
        error "Check your internet connection or visit:"
        error "https://github.com/HendryAvila/sdd-hoffy/releases"
        exit 1
    fi

    echo "$version"
}

# --- Download ---

download_binary() {
    local version="$1"
    local os="$2"
    local arch="$3"
    local install_dir="$4"

    local ext="tar.gz"
    local archive_name="sdd-hoffy_${version}_${os}_${arch}.${ext}"
    local url="https://github.com/HendryAvila/sdd-hoffy/releases/download/v${version}/${archive_name}"

    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap "rm -rf '$tmp_dir'" EXIT

    info "Downloading sdd-hoffy v${version} for ${os}/${arch}..."
    printf "  ${DIM}%s${NC}\n" "$url"

    if command -v curl &>/dev/null; then
        if ! curl -sSL --fail -o "${tmp_dir}/${archive_name}" "$url"; then
            error "Download failed!"
            error "The file might not exist for your platform (${os}/${arch})."
            error "Check available downloads: https://github.com/HendryAvila/sdd-hoffy/releases/tag/v${version}"
            exit 1
        fi
    else
        if ! wget -q -O "${tmp_dir}/${archive_name}" "$url"; then
            error "Download failed!"
            error "Check available downloads: https://github.com/HendryAvila/sdd-hoffy/releases/tag/v${version}"
            exit 1
        fi
    fi

    info "Extracting..."
    tar -xzf "${tmp_dir}/${archive_name}" -C "${tmp_dir}"

    # Find the binary (could be in root or a subdirectory)
    local binary_path
    binary_path=$(find "${tmp_dir}" -name "sdd-hoffy" -type f | head -1)

    if [ -z "$binary_path" ]; then
        error "Could not find sdd-hoffy binary in the downloaded archive."
        exit 1
    fi

    chmod +x "$binary_path"

    # Install to the target directory
    if [ "$install_dir" = "/usr/local/bin" ] && [ ! -w "$install_dir" ]; then
        info "Installing to ${install_dir} (requires sudo)..."
        sudo mv "$binary_path" "${install_dir}/sdd-hoffy"
    else
        mkdir -p "$install_dir"
        mv "$binary_path" "${install_dir}/sdd-hoffy"
    fi

    success "Installed sdd-hoffy to ${install_dir}/sdd-hoffy"
}

# --- Install directory ---

choose_install_dir() {
    # Try /usr/local/bin first (standard, in PATH by default)
    if [ -d "/usr/local/bin" ]; then
        echo "/usr/local/bin"
        return
    fi

    # Fallback to ~/.local/bin (no sudo needed)
    local local_bin="${HOME}/.local/bin"
    mkdir -p "$local_bin"
    echo "$local_bin"
}

check_path() {
    local install_dir="$1"

    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$install_dir"; then
        warn "${install_dir} is not in your PATH."
        printf "\n"
        info "Add it to your shell config:"
        printf "\n"

        local shell_name
        shell_name="$(basename "${SHELL:-/bin/bash}")"

        local rc_file
        case "$shell_name" in
            zsh)  rc_file="~/.zshrc" ;;
            fish) rc_file="~/.config/fish/config.fish" ;;
            *)    rc_file="~/.bashrc" ;;
        esac

        if [ "$shell_name" = "fish" ]; then
            printf "  ${BOLD}echo 'set -gx PATH %s \$PATH' >> %s${NC}\n" "$install_dir" "$rc_file"
        else
            printf "  ${BOLD}echo 'export PATH=\"%s:\$PATH\"' >> %s${NC}\n" "$install_dir" "$rc_file"
        fi
        printf "  ${BOLD}source %s${NC}\n\n" "$rc_file"
    fi
}

# --- Post-install verification ---

verify_install() {
    local install_dir="$1"
    local binary="${install_dir}/sdd-hoffy"

    if [ ! -x "$binary" ]; then
        error "Installation verification failed — binary not found or not executable."
        exit 1
    fi

    local version_output
    version_output=$("$binary" --version 2>&1 || true)

    if echo "$version_output" | grep -q "sdd-hoffy"; then
        success "Verification passed: ${version_output}"
    else
        warn "Binary exists but version check returned unexpected output."
        warn "Output: ${version_output}"
    fi
}

# --- Main ---

main() {
    print_banner

    step "Detecting system"
    local os arch
    os=$(detect_os)
    arch=$(detect_arch)
    success "Detected: ${os}/${arch}"

    # Check for WSL
    if [ "$os" = "linux" ] && grep -qi microsoft /proc/version 2>/dev/null; then
        info "Running inside WSL (Windows Subsystem for Linux)"
    fi

    step "Fetching latest version"
    local version
    version=$(get_latest_version)
    success "Latest version: v${version}"

    step "Installing"
    local install_dir
    install_dir=$(choose_install_dir)
    download_binary "$version" "$os" "$arch" "$install_dir"

    step "Verifying installation"
    verify_install "$install_dir"
    check_path "$install_dir"

    # Done!
    printf "\n"
    printf "  ${BOLD}${GREEN}╔═══════════════════════════════════════════╗${NC}\n"
    printf "  ${BOLD}${GREEN}║                                           ║${NC}\n"
    printf "  ${BOLD}${GREEN}║   🎉 SDD-Hoffy installed successfully!    ║${NC}\n"
    printf "  ${BOLD}${GREEN}║                                           ║${NC}\n"
    printf "  ${BOLD}${GREEN}╚═══════════════════════════════════════════╝${NC}\n"
    printf "\n"
    printf "  ${BOLD}Next: Add SDD-Hoffy to your AI tool's MCP config:${NC}\n\n"
    printf "  ${DIM}Claude Code (.claude/settings.json), Cursor, VS Code Copilot (.vscode/mcp.json),${NC}\n"
    printf "  ${DIM}Gemini CLI:${NC}\n\n"
    printf "  ${CYAN}%s${NC}\n" '{'
    printf "  ${CYAN}%s${NC}\n" '  "mcpServers": {'
    printf "  ${CYAN}%s${NC}\n" '    "sdd-hoffy": {'
    printf "  ${CYAN}%s${NC}\n" '      "command": "sdd-hoffy",'
    printf "  ${CYAN}%s${NC}\n" '      "args": ["serve"]'
    printf "  ${CYAN}%s${NC}\n" '    }'
    printf "  ${CYAN}%s${NC}\n" '  }'
    printf "  ${CYAN}%s${NC}\n" '}'
    printf "\n"
    printf "  ${DIM}OpenCode (~/.config/opencode/opencode.json, inside the \"mcp\" key):${NC}\n\n"
    printf "  ${CYAN}%s${NC}\n" '"sdd-hoffy": {'
    printf "  ${CYAN}%s${NC}\n" '  "type": "local",'
    printf "  ${CYAN}%s${NC}\n" '  "command": ["sdd-hoffy", "serve"],'
    printf "  ${CYAN}%s${NC}\n" '  "enabled": true'
    printf "  ${CYAN}%s${NC}\n" '}'
    printf "\n"
    printf "  ${BOLD}What's next?${NC}\n\n"
    printf "    1. Add the JSON snippet above to your AI tool's MCP config\n"
    printf "    2. Use the ${BOLD}/sdd-start${NC} prompt to begin\n"
    printf "    3. Describe your idea — SDD-Hoffy will guide you\n"
    printf "\n"
    printf "  ${BOLD}Useful commands:${NC}\n\n"
    printf "    ${CYAN}sdd-hoffy serve${NC}     Start the MCP server\n"
    printf "    ${CYAN}sdd-hoffy update${NC}    Update to the latest version\n"
    printf "    ${CYAN}sdd-hoffy --help${NC}    Show help\n"
    printf "\n"
    printf "  ${DIM}Docs: https://github.com/HendryAvila/sdd-hoffy${NC}\n"
    printf "  ${DIM}Star ⭐ if you find it useful!${NC}\n\n"
}

main "$@"
