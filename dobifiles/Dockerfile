
ARG     GOLANG_VERSION
FROM    golang:${GOLANG_VERSION:-1.12-alpine} as golang
RUN     apk add -U curl git bash
ENV     CGO_ENABLED=0 \
        PS1="# " \
        GO111MODULE=on
WORKDIR /go/src/gotest.tools/gotestsum

FROM    golang as tools
RUN     go get github.com/dnephin/filewatcher@v0.3.2
RUN     wget -O- -q https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s && \
            mv bin/golangci-lint /go/bin

FROM    golang as dev
COPY    --from=tools /go/bin/filewatcher /usr/bin/filewatcher
COPY    --from=tools /go/bin/golangci-lint /usr/bin/golangci-lint

FROM    dev as dev-with-source
COPY    . .
