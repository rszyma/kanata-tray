# Feature: control server

Control server is a feature in kanata-tray, to run TCP control server.
It allows to programatically send commands to kanata-tray.
For example it can be used to remotely to stop/start a preset (e.g. by mapping it to a keybind).

### Related config options:

- `general.control_server_enable` - (default: `false`) - Enables the control server feature.
- `general.control_server_port` - (default: `8100`) - TCP port to listen on. It's ran on `localhost` address.

### Available endpoints

- `/stop/{preset_name}` - Stops a specific preset by a name.
- `/stop_all` - Stops all running presets.
- `/start/{preset_name}` - Runs a specific preset by a name.
- `/start_all_default` - Runs all presets that have `autorun = true`.
- `/toggle/{preset_name}` - Stops or starts a specific preset by a name.
- `/toggle_all_default` - Stops or starts all presets that have `autorun = true`.

Generally, if a preset is already running and `/start*` endpoint is called on it,
nothing will happen. Similarly, stopping already stopped preset will do nothing.

### Usage

Send a HTTP request to one of the endpoints. Any HTTP method is allowed.

Possible status codes of response are 200, 400, 500. All responses will be in JSON format
and generally will look like this:

```
{
  "IsSuccess": true,
  "Message": "started all default presets"
}
```

`IsSuccess` will always be present. `Message` might optionally be present or not,
describing what happened. `Data` is another field that might be present if relevant.

### Examples using `curl`:

- `curl "localhost:8100/start/my_preset_1"`
- `curl "localhost:8100/toggle_all_default"`
