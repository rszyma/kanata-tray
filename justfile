
_:
    @just -l --unsorted

# build and run with "no_gtk_overlay" tag (clean build a lot faster compared to building with gtk)
run_minimal:
    CGO_ENABLED=1 GO111MODULE=on go run -tags=no_gtk_overlay .

# build and run without "no_gtk_overlay" feature 
run:
    CGO_ENABLED=1 GO111MODULE=on go run .

[linux]
build_release tags="," version="latest":
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
build_release tags="," version="latest":
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
