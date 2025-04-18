{
  buildGoModule,
  lib,
  hostPlatform,
  build-deps,
  runtime-deps,
  pkgs,
  self,
  ...
}:

buildGoModule {
  name = "kanata-tray";
  src = lib.cleanSource ./..;
  vendorHash = "sha256-7+aSSvfOQKXwBh132bvz9cRB4bwTv7E/KYV7MJQ7OYU==";
  env = {
    CGO_ENABLED = 1;
    GO111MODULE = "on";
    GOOS = if hostPlatform.isDarwin then "darwin" else "linux";
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
}