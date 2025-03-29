{
  description = "Flake for kanata-tray";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }: flake-utils.lib.eachDefaultSystem
    (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        inherit (pkgs.stdenvNoCC) hostPlatform;
        runtime-deps = pkgs.lib.optionals (hostPlatform.isLinux) [pkgs.libayatana-appindicator pkgs.gtk3];
        build-deps = [ pkgs.pkg-config ];
      in
      rec {
        packages.default = packages.kanata-tray;
        packages.kanata-tray =
          pkgs.buildGoModule {
            name = "kanata-tray";
            src = pkgs.lib.cleanSource ./.;
            vendorHash = "sha256-2rR368zzVFhgntVDynXCYNWzM4jalsnDRGaUo81bqIE=";
            env = { CGO_ENABLED = 1; } // pkgs.lib.attrsets.optionalAttrs (hostPlatform.isDarwin) {
              GOOS = "darwin";
              GO111MODULE = "on";
            };

            flags = [ "-trimpath" ];
            ldflags = [
              "-s"
              "-w"
              "-X main.buildVersion=nix"
              "-X main.buildHash=${self.shortRev or self.dirtyShortRev or "unknown"}"
              "-X main.buildDate=unknown"
            ];
            nativeBuildInputs = build-deps;
            buildInputs = runtime-deps ++ [ pkgs.makeWrapper ];
            postInstall = ''
              wrapProgram $out/bin/kanata-tray --set KANATA_TRAY_LOG_DIR /tmp --prefix PATH : $out/bin
            '';
            meta = with pkgs.lib; {
              description = "Tray Icon for Kanata";
              longDescription = ''
                A simple wrapper for kanata to control it from tray icon.
                Works on Windows, Linux and macOS.
              '';
              homepage = "https://github.com/rszyma/kanata-tray";
              license = licenses.gpl3;
              platforms = platforms.unix;
            };
          };

        devShells.default = pkgs.mkShell
          {
            packages = with pkgs;
              build-deps
              ++ runtime-deps
              ++ [
                # converting png -> ico
                #  convert input.png -define icon:auto-resize=48,32,16 output.ico
                imagemagick
              ];
          };
      }
    );
}