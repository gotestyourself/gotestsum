# Running without Go

`gotestsum` may be run without Go as long as the package to be tested has
already been compiled using `go test -c`, and the `test2json` tool is available.

The `test2json` tool can be compiled from the Go source tree so that it can be distributed to the environment that needs it.

```sh
GOVERSION=1.17.6
OS=$(uname -s | sed 's/.*/\L&/')
mkdir -p gopath
GOPATH=$(realpath gopath)
HOME=$(realpath ./)
curl -L --silent https://go.dev/dl/go${GOVERSION}.${OS}-amd64.tar.gz | tar xz -C ./
env HOME=$HOME GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOPATH=$GOPATH ./go/bin/go build -o test2json -ldflags="-s -w" cmd/test2json
mv test2json /usr/local/bin/test2json
```

Or if you have Go installed already:

```sh
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o test2json -ldflags="-s -w" cmd/test2json
mv test2json /usr/local/bin/test2json
```

Example: running without a Go installation
```
export GOVERSION=1.13
gotestsum --raw-command -- test2json -t -p pkgname ./binary.test -test.v
```
