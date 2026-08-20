# Feature: presets autorun

`preset.<name>.autorun` - Allows automatically running a preset at kanata-tray startup.
Accepts `true`/`false` or a string with more complex logic.
Disabled by default.

### Examples:

```toml
[presets.'preset-1']
autorun = true # A simple, unconditional autorun.

[presets.'preset-2']
autorun = 'linux' # Autorun only on linux.

[presets.'preset-3']
autorun = 'darwin || windows' # Autorun either on macOS or on Windows, but not Linux.

[presets.'preset-4']
autorun = '!linux && env("HOSTNAME") == "laptop1"' # Autorun only if not Linux AND hostname is "laptop1".
```

# expr-lang spec

The expression language used is an embedded mini [DSL](https://en.wikipedia.org/wiki/Domain-specific_language), called expr-lang, full docs available here: https://expr-lang.org/docs/language-definition.

Notably, kanata-tray provides you access to checking OS type: `windows`/`linux`/`darwin` and environment variables: `env("MY_ENV")` within expressions.