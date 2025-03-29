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
      {
        packages.kanata-tray = pkgs.callPackage ./nix/package.nix { inherit build-deps runtime-deps hostPlatform self; };
        packages.default = self.packages.${system}.kanata-tray;

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
    ) // {
      homeManagerModules.kanata-tray = import ./nix/hmModule.nix self;
      homeManagerModules.default = self.homeManagerModules.kanata-tray;
    };
}