set windows-powershell := true
 
just:
    just -l

run:
    CGO_ENABLED=1 GO111MODULE=on go run .

build_release tags="gtk_overlay" version="latest":
	just _build_release_{{os()}} {{tags}} {{version}}

_build_release_linux tags version:
    GOOS=linux CGO_ENABLED=1 GO111MODULE=on go build -tags={{tags}} -ldflags "-s -w -X 'main.buildVersion={{version}}' -X 'main.buildHash=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u)'" -trimpath -o dist/kanata-tray

# CGO cross-compilation is not supported with 'gtk_overlay' tag 
_build_release_windows tags version:
    GOOS=windows CGO_ENABLED=1 GO111MODULE=on go build -tags={{tags}} -ldflags "-H=windowsgui -s -w -X 'main.buildVersion={{version}}' -X 'main.buildHash=$(git rev-parse HEAD)' -X 'main.buildDate=$(date -u)'" -trimpath -o dist/kanata-tray.exe

# e.g. "push_tag v0.1.0"
push_tag tag:
    git tag {{tag}}
    git push --tags
