# Running kanata-tray as a macOS Launch Agent (with mouse support)

This guide explains how to run kanata-tray automatically at login via a macOS Launch Agent, with full mouse support (movement, clicks, scroll wheel) working through kanata.

## Quick setup

Run the automated setup script:

```bash
./scripts/setup-macos-launchagent.sh
```

The script handles everything except granting Accessibility permission, which requires the macOS GUI. It will print instructions for that final step.

The rest of this document explains what the script does and why, which is useful if you need to troubleshoot or customize the setup.

## Background

On macOS, kanata uses two different mechanisms for output:

| Output | Mechanism | Requirement |
|--------|-----------|-------------|
| Keyboard | Karabiner-DriverKit-VirtualHIDDevice | Root access (`sudo`) |
| Mouse (movement, clicks, scroll) | CGEvent via CoreGraphics (Quartz) | **Accessibility** permission (TCC) |

The Accessibility permission is evaluated against the **responsible process**, the ancestor that originated the process chain, not the process that posts the event. This is why a simple `sudo kanata-tray-macos` plist doesn't work for mouse: `launchd` would spawn `/usr/bin/sudo` as the responsible process, and granting Accessibility to `/usr/bin/sudo` is a bad idea.

The solution is a small launcher binary that stays alive as the responsible process, gets the Accessibility permission, and spawns `sudo kanata-tray-macos` as a child.

> **Note:** A Launch**Daemon** will not work for this use case. Daemons run outside the user's GUI session (Aqua), so neither the tray icon nor CGEvents can reach the desktop.

## Prerequisites

- macOS with Karabiner-DriverKit-VirtualHIDDevice installed
- kanata installed (e.g. via Homebrew: `brew install kanata`)
- kanata-tray binary at `/usr/local/bin/kanata-tray-macos` (download from [releases](https://github.com/rszyma/kanata-tray/releases/latest))
- Xcode Command Line Tools (`xcode-select --install`)

## What the script does

### 1. Compiles and installs the launcher binary

A small C program (`kanata-tray-launcher`) that uses `posix_spawn` + `waitpid` to run `sudo -n kanata-tray-macos` as a child process. It deliberately does **not** use `exec`, if it did, the process image would be replaced by `sudo`, and TCC would evaluate Accessibility against `/usr/bin/sudo` instead of the launcher.

The launcher forwards `SIGTERM`/`SIGINT` to the child process, so `launchctl bootout` cleanly stops the entire chain.

It is ad-hoc signed and installed to `/usr/local/bin/kanata-tray-launcher`.

### 2. Configures sudoers

Creates `/etc/sudoers.d/kanata-tray` allowing your user to run `kanata-tray-macos` as root without a password. The launcher uses `sudo -n` (non-interactive), so if this rule is missing, it fails immediately with a clear error instead of hanging.

### 3. Creates and loads the Launch Agent plist

Installs `~/Library/LaunchAgents/com.kanata-tray.plist` with:
- `LimitLoadToSessionType: Aqua`, only runs in GUI sessions
- `ProcessType: Interactive`, appropriate scheduling priority
- `KeepAlive` and `RunAtLoad`, starts at login, restarts on crash
- `ThrottleInterval: 20`, prevents rapid restart loops

### 4. Manual step: grant Accessibility permission

This is the only step that requires manual intervention:

1. Open **System Settings > Privacy & Security > Accessibility**
2. Click **"+"**
3. Press **Cmd+Shift+G** and type `/usr/local/bin/kanata-tray-launcher`
4. Enable the toggle

Then restart the agent:

```bash
launchctl kickstart -k gui/$(id -u)/com.kanata-tray
```

## Making log files readable (optional)

When running via `sudo`, kanata-tray's preset log files (created in `/private/tmp/`) are owned by root. You can fix this with a [post-start hook](./hooks.md) in your `kanata-tray.toml`:

```toml
[defaults.hooks]
post-start = [
    "chmod 666 /private/tmp/kanata_lastrun_*"
]
```

## Useful commands

```bash
# Check agent status
launchctl print gui/$(id -u)/com.kanata-tray

# Stop and unload the agent
launchctl bootout gui/$(id -u)/com.kanata-tray

# Reload after plist changes
launchctl bootout gui/$(id -u)/com.kanata-tray
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.kanata-tray.plist

# Watch logs
tail -f /tmp/kanata-tray.err.log
```

## Maintenance notes

- **Upgrading kanata** (e.g. via Homebrew) does not affect the Accessibility permission, it's granted to the launcher, not kanata itself.
- **Upgrading kanata-tray-macos**: if you have a SHA-256 pin in your sudoers rule, update it to match the new binary.
- **Recompiling the launcher**: you must re-grant Accessibility permission (remove and re-add in System Settings). macOS identifies ad-hoc signed binaries by their cdhash.
- **Mouse still not working**: as a last resort, also add `/opt/homebrew/bin/kanata` to the Accessibility list. The silent failure of `CGEventPost` without Accessibility permission is standard macOS behavior, there will be no errors in the logs.
