#!/usr/bin/env bash
# SDD-Hoffy Installer
# One-liner: curl -sSL https://raw.githubusercontent.com/HendryAvila/sdd-hoffy/main/install.sh | bash
#
# This script:
#   1. Detects your OS and architecture
#   2. Downloads the latest sdd-hoffy binary from GitHub
#   3. Installs it to your PATH
#   4. Optionally configures your AI tool's MCP settings
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

# --- Interactive input ---
# When running via curl | bash, stdin is the script itself.
# Reopen /dev/tty so we can ask the user questions regardless.
if [ ! -t 0 ] && [ -e /dev/tty ]; then
    exec < /dev/tty
fi

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

# --- MCP Configuration Wizard ---

configure_mcp() {
    step "MCP Configuration"
    printf "\n"
    printf "  SDD-Hoffy works with any AI coding tool that supports MCP.\n"
    printf "  Want to configure it now? ${DIM}(You can always do this later)${NC}\n\n"

    printf "  Which tool do you use?\n\n"
    printf "    ${BOLD}1${NC}) Claude Code (Anthropic CLI)\n"
    printf "    ${BOLD}2${NC}) Cursor\n"
    printf "    ${BOLD}3${NC}) VS Code + GitHub Copilot\n"
    printf "    ${BOLD}4${NC}) OpenCode\n"
    printf "    ${BOLD}5${NC}) Gemini CLI\n"
    printf "    ${BOLD}6${NC}) Skip — I'll configure it myself\n"
    printf "\n"

    local choice
    printf "  Enter your choice ${DIM}[1-6]${NC}: "
    read -r choice

    case "$choice" in
        1) configure_claude_code ;;
        2) configure_cursor ;;
        3) configure_vscode_copilot ;;
        4) configure_opencode ;;
        5) configure_gemini_cli ;;
        6|"")
            info "No problem! You can configure MCP manually later."
            print_manual_config
            ;;
        *)
            warn "Invalid choice. Skipping MCP configuration."
            print_manual_config
            ;;
    esac
}

# Adds sdd-hoffy to an existing JSON MCP config file.
# Creates the file if it doesn't exist.
# $1 = file path
# $2 = JSON key for the servers object (e.g., "mcpServers" or "servers")
add_mcp_server_to_config() {
    local config_file="$1"
    local servers_key="$2"
    local config_dir
    config_dir=$(dirname "$config_file")

    # Create directory if needed
    mkdir -p "$config_dir"

    local sdd_entry
    sdd_entry=$(cat <<'ENTRY'
{"command":"sdd-hoffy","args":["serve"]}
ENTRY
)

    if [ -f "$config_file" ]; then
        # File exists — check if sdd-hoffy is already configured
        if grep -q '"sdd-hoffy"' "$config_file" 2>/dev/null; then
            success "SDD-Hoffy is already configured in ${config_file}"
            return
        fi

        # Check if we have jq or python3 for safe JSON manipulation
        if command -v jq &>/dev/null; then
            local tmp_file="${config_file}.tmp"
            jq --arg key "$servers_key" \
               ".[\$key][\"sdd-hoffy\"] = {\"command\": \"sdd-hoffy\", \"args\": [\"serve\"]}" \
               "$config_file" > "$tmp_file" && mv "$tmp_file" "$config_file"
        elif command -v python3 &>/dev/null; then
            python3 -c "
import json, sys
with open('$config_file', 'r') as f:
    data = json.load(f)
data.setdefault('$servers_key', {})['sdd-hoffy'] = {'command': 'sdd-hoffy', 'args': ['serve']}
with open('$config_file', 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
"
        else
            warn "Neither jq nor python3 found — can't safely modify existing config."
            warn "Please add sdd-hoffy manually to ${config_file}"
            print_manual_config
            return
        fi
    else
        # File doesn't exist — create it
        cat > "$config_file" <<NEWCONFIG
{
  "${servers_key}": {
    "sdd-hoffy": {
      "command": "sdd-hoffy",
      "args": ["serve"]
    }
  }
}
NEWCONFIG
    fi

    success "Configured SDD-Hoffy in ${config_file}"
}

configure_claude_code() {
    info "Configuring for Claude Code..."

    local config_file="${HOME}/.claude.json"

    # Claude Code uses ~/.claude.json with mcpServers key
    add_mcp_server_to_config "$config_file" "mcpServers"

    printf "\n"
    info "Claude Code will detect sdd-hoffy on next startup."
    info "Try it: ${BOLD}claude${NC} and then use the /sdd-start prompt."
}

configure_cursor() {
    info "Configuring for Cursor..."

    local config_file="${HOME}/.cursor/mcp.json"

    add_mcp_server_to_config "$config_file" "mcpServers"

    printf "\n"
    info "Restart Cursor to activate sdd-hoffy."
}

configure_vscode_copilot() {
    info "Configuring for VS Code + GitHub Copilot..."

    # VS Code MCP config goes in the workspace .vscode directory
    local config_file=".vscode/mcp.json"

    printf "\n"
    printf "  VS Code MCP config is ${BOLD}per-project${NC}.\n"
    printf "  Create the config in your current directory? ${DIM}(%s)${NC}\n" "$(pwd)"
    printf "  ${DIM}[Y/n]${NC}: "
    read -r yn

    case "$yn" in
        [Nn]*)
            info "Skipping. You can add it manually to any project's .vscode/mcp.json"
            print_manual_config
            return
            ;;
    esac

    add_mcp_server_to_config "$config_file" "servers"

    printf "\n"
    info "Open this project in VS Code to use sdd-hoffy with Copilot."
}

configure_opencode() {
    info "Configuring for OpenCode..."

    local config_file="${HOME}/.config/opencode/config.json"

    add_mcp_server_to_config "$config_file" "mcpServers"

    printf "\n"
    info "Restart OpenCode to activate sdd-hoffy."
}

configure_gemini_cli() {
    info "Configuring for Gemini CLI..."

    local config_file="${HOME}/.gemini/settings.json"

    add_mcp_server_to_config "$config_file" "mcpServers"

    printf "\n"
    info "Restart Gemini CLI to activate sdd-hoffy."
}

print_manual_config() {
    printf "\n"
    printf "  ${DIM}Add this to your AI tool's MCP configuration:${NC}\n\n"
    printf "  ${BOLD}{\n"
    printf "    \"mcpServers\": {\n"
    printf "      \"sdd-hoffy\": {\n"
    printf "        \"command\": \"sdd-hoffy\",\n"
    printf "        \"args\": [\"serve\"]\n"
    printf "      }\n"
    printf "    }\n"
    printf "  }${NC}\n\n"
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

    # MCP configuration wizard
    configure_mcp

    # Done!
    printf "\n"
    printf "  ${BOLD}${GREEN}╔═══════════════════════════════════════════╗${NC}\n"
    printf "  ${BOLD}${GREEN}║                                           ║${NC}\n"
    printf "  ${BOLD}${GREEN}║   🎉 SDD-Hoffy installed successfully!    ║${NC}\n"
    printf "  ${BOLD}${GREEN}║                                           ║${NC}\n"
    printf "  ${BOLD}${GREEN}╚═══════════════════════════════════════════╝${NC}\n"
    printf "\n"
    printf "  ${BOLD}What's next?${NC}\n\n"
    printf "    1. Open your AI coding tool\n"
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
