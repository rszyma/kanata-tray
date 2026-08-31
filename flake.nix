{
  description = "Flake for kanata-tray";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    {
      self,
      nixpkgs,
      ...
    }:
    let
      lib = nixpkgs.lib;
      lib0 = (import ./nix/lib0.nix lib);
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      pkgsFor = system: nixpkgs.legacyPackages.${system};
      packageFor = system: (pkgsFor system).callPackage ./nix/package.nix { inherit self; };
    in
    lib0.deepMergeList (
      lib.forEach supportedSystems (
        system:
        let
          pkgs = pkgsFor system;
          pkg = packageFor system;
        in
        {
          packages.${system} = {
            kanata-tray = pkg;
            default = pkg;
          };
          devShells.${system}.default = pkgs.mkShell {
            packages =
              pkg.buildInputs
              ++ pkg.nativeBuildInputs
              ++ [
                pkgs.go
                # converting png -> ico
                #  convert input.png -define icon:auto-resize=48,32,16 output.ico
                pkgs.imagemagick
              ];
          };

          formatter.${system} = nixpkgs.legacyPackages.${system}.nixfmt-tree;
        }
      )
    )
    // {
      homeManagerModules.kanata-tray = import ./nix/hmModule.nix self;
      homeManagerModules.default = self.homeManagerModules.kanata-tray;
    };

  # Does this even work?
  # It's not that heavy to compile, cache is not really needed.
  # nixConfig = {
  #   extra-substituters = [ "https://rszyma.cachix.org" ];
  #   extra-trusted-public-keys = [ "rszyma.cachix.org-1:L3LKXbrUk+OfUBXj2JjxNrq23Z2BccrgDm/S2r012tg=" ];
  # };
}
