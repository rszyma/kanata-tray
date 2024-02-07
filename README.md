# kanata-tray

A simple wrapper for [kanata](https://github.com/jtroo/kanata) to control it from tray icon. 
Should work on Windows and Linux.

## Features

- Tray icon with buttons for starting/stopping kanata
- Easy switching between multiple kanata configurations and versions.
- Works out-of-the box with no configuration, but can be configured with toml file.

## Configuration

An example configuration file.

Note that, you don't need to provide config file at all. In that case the defaults will be applied.

```toml
# default: []
configurations = [
    "~/.config/kanata/kanata.kbd",
    "~/.config/kanata/test.kbd",
]

# default: []
executables = [
    "~/.config/kanata/kanata", 
    "~/.config/kanata/kanata-debug",
]

[general]
include_executables_from_system_path = false # default: true
include_configs_from_default_locations = false # default: true
launch_on_start = true # default: true
```

The above config should be pretty self-explanatory.

There are a few things worth noting about `launch_on_start = true` though:
- If `configurations` is non-empty, the first item from it will be launched.
- Otherwise it falls back to a first kanata configs found by `include_configs_from_default_locations` option, if enabled.
- Otherwise, if no configs available, it won't auto-launch anything.
- Similar thing with kanata executables (see options `executables` and `include_executables_from_system_path`)

## Installation

Make sure to install required packages first:

Arch :
```bash
pacman -S libayatana-appindicator
# also: gcc, libgtk-3-dev
```

Ubuntu:
```bash
sudo apt-get install gcc libgtk-3-dev libayatana-appindicator3-dev
```

## Building 

See [justfile](./justfile)
 