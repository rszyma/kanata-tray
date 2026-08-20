package defaultconfig

import _ "embed"

//go:embed default_config.toml
var DefaultConfigContent string
