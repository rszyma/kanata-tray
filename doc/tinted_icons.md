# Feature: template icons on macOS

On macOS, an icon whose filename ends with `Template` (before the extension) is set as a
[template image](https://developer.apple.com/documentation/appkit/nsimage/istemplate): macOS ignores its colors and tints the shape from the alpha channel, so it follows a light or dark menu bar the same way the system clock and Control Center icons do. Icons can be mixed freely; only the ones named `...Template` are tinted.

Such an icon should be monochrome (e.g. solid black) with a transparent background. Everything
that isn't transparent gets tinted, so a colored or fully opaque icon will show up as a solid
blob.

It works for both layer icons and status icons:

```toml
[defaults.layer_icons]
mouse = 'mouseTemplate.png' # tinted to match the menu bar
qwerty = 'qwerty.png'       # used as-is
```

```
status_icons/
  defaultTemplate.png  # tinted
  pause.ico            # used as-is
```

For status icons, a `<status>Template.*` file takes precedence over a plain `<status>.*` one,
so there's no need to delete the default icons written on first run.

On Linux and Windows the `Template` suffix has no special meaning and such icons are drawn
as-is.