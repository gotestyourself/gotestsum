ARG     GOLANG_VERSION
FROM    golang:${GOLANG_VERSION:-1.10-alpine}

RUN     apk add -U curl git bash

ARG     FILEWATCHER_SHA=v0.2.1
RUN     go get -d github.com/dnephin/filewatcher && \
        cd /go/src/github.com/dnephin/filewatcher && \
        git checkout -q "$FILEWATCHER_SHA" && \
        go build -v -o /usr/bin/filewatcher . && \
        rm -rf /go/src/* /go/pkg/* /go/bin/*

ARG     DEP_TAG=v0.4.1
RUN     go get -d github.com/golang/dep/cmd/dep && \
        cd /go/src/github.com/golang/dep && \
        git checkout -q "$DEP_TAG" && \
        go build -v -o /usr/bin/dep ./cmd/dep && \
        rm -rf /go/src/* /go/pkg/* /go/bin/*

ENV     PS1="# "
WORKDIR /go/src/gotest.tools/gotestsum
ENV     CGO_ENABLED=0
