
run:
    CGO_ENABLED=1 GO111MODULE=on go run . -ldflags "-H=windowsgui"

build_release_windows:
    GOOS=windows CGO_ENABLED=1 GO111MODULE=on go build -ldflags "-H=windowsgui -s -w" -trimpath -o dist/kanata-tray.exe

build_release_linux:
    GOOS=linux CGO_ENABLED=1 GO111MODULE=on go build -ldflags "-s -w" -trimpath -o dist/kanata-tray
