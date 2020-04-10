#!/usr/bin/env bash

source .plsdo.sh

binary() {
    mkdir -p dist
    go build -o dist/gotestsum .
}

update-golden() {
    gotestsum -- ./testjson ./internal/junitxml -test.update-golden
}

lint() {
    golangci-lint run -v
}

go-mod-tidy() {
    go mod tidy
    git diff --stat --exit-code go.mod go.sum
}

shell() {
    local tag; tag="$(_docker-build-dev)"
    docker run \
        --tty --interactive --rm \
        -v "$PWD:/work" \
        -v ~/.cache/go-build:/root/.cache/go-build \
        -v ~/go/pkg/mod:/go/pkg/mod \
        -w /work \
        "$tag" bash
}

_docker-build-dev() {
    set -e
    local idfile=.plsdo/docker-build-dev-image-id
    local dockerfile=Dockerfile
    local tag=gotest.tools/gotestsum/builder
    if [ -f "$idfile" ] && [ "$dockerfile" -ot "$idfile" ]; then
        echo "$tag"
        return 0
    fi

    mkdir -p .plsdo
    >&2 docker build \
        --iidfile "$idfile"  \
        --file "$dockerfile" \
        --tag "$tag" \
        --build-arg "UID=$UID" \
        --target "dev" \
        .plsdo
    echo "$tag"
}

_plsdo_run "$@"
