# macOS Launch Agent Setup Guide (Community)

This guide provides a community-supported method to run kanata-tray automatically at login via a macOS Launch Agent, with full mouse support (movement, clicks, scroll wheel).

> **Note:** Because macOS handles Accessibility permissions strictly, this setup uses a custom launcher binary. See [caveats.md](./caveats.md) for a technical explanation.

## Quick setup

An automated setup script has been provided as a GitHub Gist:
[kanata-tray macOS launch agent setup](https://gist.github.com/jvlara/d54e12beea5e27b54546b52b57f53096)

Download and run the script:

```bash
curl -O https://gist.githubusercontent.com/jvlara/d54e12beea5e27b54546b52b57f53096/raw/setup-macos-launchagent.sh
chmod +x setup-macos-launchagent.sh
./setup-macos-launchagent.sh
```

The script handles everything except granting Accessibility permission, which requires the macOS GUI. It will print instructions for that final step.

## Prerequisites

- macOS with Karabiner-DriverKit-VirtualHIDDevice installed
- kanata installed (e.g. via Homebrew: `brew install kanata`)
- kanata-tray binary at `/usr/local/bin/kanata-tray-macos` (download from [releases](https://github.com/rszyma/kanata-tray/releases/latest))
- Xcode Command Line Tools (`xcode-select --install`)

## Manual step: grant Accessibility permission

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

When running via `sudo`, kanata-tray's preset log files (created in `/private/tmp/`) are owned by root. You can fix this with a [post-start hook](../hooks.md) in your `kanata-tray.toml`:

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
