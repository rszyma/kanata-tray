
_:
    @just -l --unsorted

# build and run without "gtk_overlay" feature
run_minimal:
    CGO_ENABLED=1 GO111MODULE=on go run .

# build and run with "gtk_overlay" feature (clean build takes 5x longer than 'run_minimal')
run:
    CGO_ENABLED=1 GO111MODULE=on go run -tags=gtk_overlay .

[linux]
build_release tags="gtk_overlay" version="latest":
    #!/bin/bash

    buildHash=$(git rev-parse HEAD)
    buildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

    ldflags="-s -w -X 'main.buildVersion={{version}}' -X 'main.buildHash=$buildHash' -X 'main.buildDate=$buildDate'"
    outputPath="dist/kanata-tray.exe"

    export GOOS="linux"
    export CGO_ENABLED="1"
    export GO111MODULE="on"
    go build -v -tags {{tags}} -ldflags "$ldflags" -trimpath -o "$outputPath"

[windows]
build_release tags="gtk_overlay" version="latest":
    #!powershell
    
    $buildHash = $(git rev-parse HEAD)
    $buildDate = Get-Date -UFormat '%Y-%m-%dT%H:%M:%SZ'

    $ldflags = "-H=windowsgui -s -w -X 'main.buildVersion={{version}}' -X 'main.buildHash=$buildHash' -X 'main.buildDate=$buildDate'"
    $outputPath = "dist\kanata-tray.exe"

    $env:GOOS = "windows"
    $env:CGO_ENABLED = "1"
    $env:GO111MODULE = "on"
    go build -v -tags {{tags}} -ldflags $ldflags -trimpath -o $outputPath

# e.g. "push_tag v0.1.0"
push_tag tag:
    git tag {{tag}}
    git push --tags
