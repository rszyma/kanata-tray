# Feature: hooks

Hooks allow running any command/program before/after starting/stopping preset. Hooks are specified per-preset.

All available hooks:
- `pre-start` - runs AFTER starting preset and BEFORE starting running kanata;
- `post-start` - runs AFTER preset starting preset AFTER kanata has been ran and set up;
- `post-start-async` - similar to `post-start`, but doesn't block and runs in the background;
- `post-stop` - runs AFTER stopping preset and AFTER kanata has been stopped;

**IMPORTANT!**: non-async hooks have maximum allowed runtime 5 seconds! Otherwise they will crash preset. Read sections below for more info.

### Example usage of hooks in config

```toml
[defaults.hooks] # hooks in "defaults" apply to all presets, similar to other items in defaults
cmd_template = ["/bin/sh", "-c", "{}"] # <- default on linux/macOS. On Windows the default is ["{}"].
pre-start = [
    # All hooks here are executed at the same time, careful for race condition!
    "./my-quick-script.bash",
    "./setup.sh & && sleep 3", # careful to not exceed the fixed 5 seconds timeout.
    "/home/rszyma/other-setup-program.exe --flag1 --flag2"
]
post-start-async = [
    # async hook can block indefinitely, it's no problem.
    # It will be eventually killed when stopping preset.
    "/usr/bin/sleep 100000"
]
post-start = []
# post-stop = []

[presets.'main']
kanata_config = '~/.config/kanata/kanata.kbd'
autorun = true

[presets.'main with helper daemon']
kanata_config = '~/.config/kanata/kanata.kbd'
# Specifitying at least 1 hook directly in a preset removes ALL hooks from defaults for this preset.
hooks.post-start-async = [
    './bin/kanata_helper_daemon.exe -p 1337 -c ./kanata.kbd'
]
```

### How hooks work

If any hook command fails, the entire preset will be marked as failed. This means, kanata process will be terminated too.
Failure is detected when program returns non-0 exit status.

Hooks accept a list of of processes to run. E.g `defaults.hooks.pre-start = ["cmd1", "cmd2"]`. If one process in a hook list fail,
all other will be allowed to finish normally, but after that, preset will be marked as failed.

Non-async (blocking) hooks (`pre-start`, `post-start`, `post-stop`) block further execution waiting for the called commands to exit.
To prevent hanging kanata-tray, these have maximum allowed runtime of 5 seconds. If any hook
If you want run long-running program from a hook, you need to either use a script that will run your command in background
e.g. `bash -c './my-long-running-program & && sleep 3'` or run it from `post-start-async`.

Async (non-blocking) hooks. Unlike non-async hooks, they don't block waiting for command program to finish, but run in background.
Currenly there's only one: `post-start-async`. It's useful when you want a neat way
to run your long-running programs, but also want to terminate it when preset exits.
