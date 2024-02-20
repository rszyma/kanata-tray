
just:
    just -l

run:
    CGO_ENABLED=1 GO111MODULE=on go run .

build_release_linux version="latest":
    GOOS=linux CGO_ENABLED=1 GO111MODULE=on go build -ldflags "-s -w -X 'main.buildVersion={{version}}' -X 'main.buildHash=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u)'" -trimpath -o dist/kanata-tray

build_release_windows version="latest":
    GOOS=windows CGO_ENABLED=1 GO111MODULE=on go build -ldflags "-H=windowsgui -s -w -X 'main.buildVersion={{version}}' -X 'main.buildHash=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u)'" -trimpath -o dist/kanata-tray.exe

# e.g. "push_tag v0.1.0"
push_tag tag:
    git tag {{tag}}
    git push --tags