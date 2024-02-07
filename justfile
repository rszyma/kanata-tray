
run:
    CGO_ENABLED=1 GO111MODULE=on go run . -ldflags "-H=windowsgui"

build:
    CGO_ENABLED=1 GO111MODULE=on go build -ldflags "-H=windowsgui" -o build
