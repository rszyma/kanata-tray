
run:
    CGO_ENABLED=1 GO111MODULE=on go run . -ldflags "-H=windowsgui"

build:
    CGO_ENABLED=1 GO111MODULE=on go build -ldflags "-H=windowsgui" -o build

gen_icon:
    #!/usr/bin/env bash
    cd ./icon &&
    GOPATH=$(go env GOPATH) ./make_icon.sh ./32x32.png