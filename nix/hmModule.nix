self:
{
  pkgs,
  lib,
  config,
  hostPlatform,
  ...
}:
let
  inherit (lib) mkIf;
  tomlFormat = pkgs.formats.toml { };
  cfg = config.programs.kanata-tray;
in
{
  options =
    let
      inherit (lib) mkOption mkEnableOption mkPackageOption;
    in
    {
      programs.kanata-tray = {
        enable = mkEnableOption "kanata-tray";
        package = mkPackageOption self.packages.${hostPlatform.system} "kanata-tray" { nullable = true; };
        settings = mkOption {
          type = tomlFormat.type;
          default = {
            defaults = {
              kanata_executable = "${self.kanata}/bin/kanata";
              hooks.cmd_template = [
                "${pkgs.bash}/bin/bash"
                "-c"
                "{}"
              ];
            };
          };
          example = lib.literalExpression ''
            {
              general = {
                allow_concurrent_presets = false;
              };
              defaults = {
                kanata_executable = "''${pkgs.kanata}/bin/kanata";
                tcp_port = 5830;
              };
            };
          '';

          description = ''
            Configuration written to
            {file}`$XDG_CONFIG_HOME/kanata-tray/kanata-tray.toml` or for darwin {file}`$HOME/Library/Application Support/kanata-tray/kanata-tray.toml`.

            See <https://github.com/rszyma/kanata-tray?tab=readme-ov-file#examples>
            for the full list of options.
          '';
        };
      };
    };

  config = mkIf (cfg.enable) {
    home.packages = [ cfg.package ];
    home.file.kanataTray = mkIf (cfg.settings != { }) {
      source = tomlFormat.generate "kanata-tray.toml" cfg.settings;
      target =
        (
          if hostPlatform.isDarwin then
            "${config.home.homeDirectory}/Library/Application Support"
          else
            "${config.xdg.configHome}"
        )
        + "/kanata-tray/kanata-tray.toml";
    };
  };

  # TOOD: VM test
}
