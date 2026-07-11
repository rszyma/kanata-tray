#!/usr/bin/env bash
set -euo pipefail

LAUNCHER_NAME="kanata-tray-launcher"
LAUNCHER_INSTALL_PATH="/usr/local/bin/${LAUNCHER_NAME}"
KANATA_TRAY_BIN="/usr/local/bin/kanata-tray-macos"
PLIST_LABEL="com.kanata-tray"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
SUDOERS_FILE="/etc/sudoers.d/kanata-tray"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m'

info()  { printf "${GREEN}==>${NC} ${BOLD}%s${NC}\n" "$*"; }
warn()  { printf "${YELLOW}Warning:${NC} %s\n" "$*"; }
error() { printf "${RED}Error:${NC} %s\n" "$*" >&2; exit 1; }

if [[ "$(uname)" != "Darwin" ]]; then
    error "This script is macOS-only."
fi

if [[ ! -x "$KANATA_TRAY_BIN" ]]; then
    error "kanata-tray-macos not found at ${KANATA_TRAY_BIN}. Download it from https://github.com/rszyma/kanata-tray/releases and place it there."
fi

if ! command -v clang &>/dev/null; then
    error "clang not found. Install Xcode Command Line Tools: xcode-select --install"
fi

TMPDIR_BUILD="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_BUILD"' EXIT

# --- Step 1: Compile the launcher ---

info "Compiling ${LAUNCHER_NAME}..."

cat > "${TMPDIR_BUILD}/${LAUNCHER_NAME}.c" << 'LAUNCHER_SOURCE'
#include <signal.h>
#include <spawn.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/wait.h>
#include <unistd.h>

static pid_t child_pid = 0;

static void forward_signal(int sig) {
    if (child_pid > 0) kill(child_pid, sig);
}

int main(void) {
    char *argv[] = {
        "/usr/bin/sudo", "-n",
        "/usr/local/bin/kanata-tray-macos",
        NULL
    };
    extern char **environ;

    signal(SIGTERM, forward_signal);
    signal(SIGINT,  forward_signal);

    int rc = posix_spawn(&child_pid, argv[0], NULL, NULL, argv, environ);
    if (rc != 0) {
        fprintf(stderr, "posix_spawn failed: %d\n", rc);
        return 1;
    }

    int status;
    waitpid(child_pid, &status, 0);
    return WIFEXITED(status) ? WEXITSTATUS(status) : 1;
}
LAUNCHER_SOURCE

clang -O2 -Wall -o "${TMPDIR_BUILD}/${LAUNCHER_NAME}" "${TMPDIR_BUILD}/${LAUNCHER_NAME}.c"
codesign -f -s - -i "com.kanata-tray.launcher" "${TMPDIR_BUILD}/${LAUNCHER_NAME}"

info "Installing ${LAUNCHER_NAME} to ${LAUNCHER_INSTALL_PATH}..."
sudo cp "${TMPDIR_BUILD}/${LAUNCHER_NAME}" "$LAUNCHER_INSTALL_PATH"
sudo chown root:wheel "$LAUNCHER_INSTALL_PATH"
sudo chmod 755 "$LAUNCHER_INSTALL_PATH"

# --- Step 2: Configure sudoers ---

CURRENT_USER="$(whoami)"

if sudo test -f "$SUDOERS_FILE" && sudo grep -q "$KANATA_TRAY_BIN" "$SUDOERS_FILE" 2>/dev/null; then
    info "Sudoers rule already exists, skipping."
else
    info "Configuring sudoers for passwordless kanata-tray-macos..."
    echo "${CURRENT_USER} ALL=(root) NOPASSWD: ${KANATA_TRAY_BIN}" | sudo tee "$SUDOERS_FILE" > /dev/null
    sudo chmod 0440 "$SUDOERS_FILE"
    if ! sudo visudo -cf "$SUDOERS_FILE" &>/dev/null; then
        sudo rm -f "$SUDOERS_FILE"
        error "Sudoers syntax check failed. The file was removed. Please configure sudoers manually."
    fi
fi

# --- Step 3: Unload existing agent if present ---

if launchctl print "gui/$(id -u)/${PLIST_LABEL}" &>/dev/null; then
    info "Unloading existing Launch Agent..."
    launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null || true
fi

# --- Step 4: Create the plist ---

info "Creating Launch Agent plist at ${PLIST_PATH}..."
mkdir -p "$(dirname "$PLIST_PATH")"

cat > "$PLIST_PATH" << PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_LABEL}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${LAUNCHER_INSTALL_PATH}</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>LimitLoadToSessionType</key>
    <string>Aqua</string>

    <key>ProcessType</key>
    <string>Interactive</string>

    <key>ThrottleInterval</key>
    <integer>20</integer>

    <key>StandardOutPath</key>
    <string>/tmp/kanata-tray.out.log</string>

    <key>StandardErrorPath</key>
    <string>/tmp/kanata-tray.err.log</string>
</dict>
</plist>
PLIST

plutil -lint "$PLIST_PATH" > /dev/null

# --- Step 5: Load the agent ---

info "Loading Launch Agent..."
launchctl bootstrap "gui/$(id -u)" "$PLIST_PATH"

# --- Step 6: Manual step ---

printf "\n"
printf "${GREEN}========================================${NC}\n"
printf "${GREEN}  Setup complete!${NC}\n"
printf "${GREEN}========================================${NC}\n"
printf "\n"
printf "${BOLD}One manual step remaining:${NC}\n"
printf "\n"
printf "  Grant Accessibility permission to the launcher:\n"
printf "\n"
printf "  1. Open ${BOLD}System Settings > Privacy & Security > Accessibility${NC}\n"
printf "  2. Click ${BOLD}\"+\"${NC}\n"
printf "  3. Press ${BOLD}Cmd+Shift+G${NC} and type: ${BOLD}%s${NC}\n" "$LAUNCHER_INSTALL_PATH"
printf "  4. Enable the toggle\n"
printf "\n"
printf "  After granting the permission, restart the agent:\n"
printf "    launchctl kickstart -k gui/\$(id -u)/%s\n" "$PLIST_LABEL"
printf "\n"
printf "${BOLD}Optional:${NC} To make kanata log files readable without sudo,\n"
printf "add this to your kanata-tray.toml:\n"
printf "\n"
printf "  [defaults.hooks]\n"
printf "  post-start = [\"chmod 666 /private/tmp/kanata_lastrun_*\"]\n"
printf "\n"
printf "Useful commands:\n"
printf "  launchctl print gui/\$(id -u)/%s      # check status\n" "$PLIST_LABEL"
printf "  launchctl bootout gui/\$(id -u)/%s     # stop & unload\n" "$PLIST_LABEL"
printf "  tail -f /tmp/kanata-tray.err.log           # watch logs\n"
