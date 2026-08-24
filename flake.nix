{
  description = "Flake for kanata-tray";

  nixConfig = {
    extra-substituters = [ "https://rszyma.cachix.org" ];
    extra-trusted-public-keys = [ "rszyma.cachix.org-1:L3LKXbrUk+OfUBXj2JjxNrq23Z2BccrgDm/S2r012tg=" ];
  };

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        package = pkgs.callPackage ./nix/package.nix { inherit self; };
      in
      {
        packages.kanata-tray = package;
        packages.default = package;

        devShells.default = pkgs.mkShell {
          packages =
            package.buildInputs
            ++ package.nativeBuildInputs
            ++ [
              pkgs.go
              # converting png -> ico
              #  convert input.png -define icon:auto-resize=48,32,16 output.ico
              pkgs.imagemagick
            ];
        };

        formatter = nixpkgs.legacyPackages.${system}.nixfmt-tree;
      }
    )
    // {
      homeManagerModules.kanata-tray = import ./nix/hmModule.nix self;
      homeManagerModules.default = self.homeManagerModules.kanata-tray;
    };
}
