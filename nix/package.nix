{
  lib,
  buildGoModule,
  makeWrapper,
  libayatana-appindicator,
  gtk3,
  stdenv,
  pkg-config,
  self ? { },
  ...
}:

buildGoModule (finalAttrs: {
  pname = "kanata-tray";
  version = "git";

  src = lib.cleanSource ./..;

  vendorHash = "sha256-X4O+6Vmei6Y/8ARLCt2MJpQT+GZyLh333R5SjegeMqc=";

  flags = [ "-trimpath" ];

  ldflags = [
    "-s"
    "-w"
    "-X main.buildVersion=${(finalAttrs.version)}"
    "-X main.buildHash=${finalAttrs.src.rev or self.shortRev or self.dirtyShortRev or "unknown"}"
    "-X main.buildDate=unknown"
  ];

  nativeBuildInputs = lib.optional stdenv.hostPlatform.isLinux pkg-config;

  buildInputs = [
    makeWrapper
  ]
  ++ lib.optionals stdenv.hostPlatform.isLinux [
    libayatana-appindicator
    gtk3
  ];

  postInstall = ''
    wrapProgram $out/bin/kanata-tray --set-default KANATA_TRAY_LOG_DIR /tmp --prefix PATH : $out/bin
  '';

  meta = with lib; {
    description = "Tray Icon for Kanata";
    longDescription = ''
      A simple wrapper for kanata to control it from tray icon.
      Works on Windows, Linux and macOS.
    '';
    homepage = "https://github.com/rszyma/kanata-tray";
    license = licenses.gpl3Plus;
    platforms = platforms.unix;
  };
})
