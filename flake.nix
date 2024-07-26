{
  description = "devshell with all required dependencies for kanata-tray";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { nixpkgs, flake-utils, ... }: flake-utils.lib.eachDefaultSystem (system:
    let
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      devShells.default = pkgs.mkShell {
        packages = with pkgs; [
          # build and runtime dependencies
          pkg-config
          libayatana-appindicator
          gtk3

          # converting png -> ico
          #  convert input.png -define icon:auto-resize=48,32,16 output.ico
          imagemagick
        ];
      };
    }
  );
}
