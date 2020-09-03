# Running without Go

`gotestsum` may be run without Go as long as the package to be tested has
already been compiled using `go test -c`, and the `test2json` tool is available.

The `test2json` tool can be compiled from the Go source tree:

```sh
export GOVERSION=1.13.5
curl -Lo go.zip "https://github.com/golang/go/archive/go${GOVERSION}.zip"
unzip go.zip
rm -f go.zip
cd go-go${GOVERSION}/src/cmd/test2json/
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" .
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

