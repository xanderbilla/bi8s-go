// Module for out-of-tree integration and e2e tests. Production code lives in
// the `app/` module; this module exists so tests can be vendored / built
// independently and so `go test ./test/...` does not pollute `app`'s module
// graph.
module github.com/xanderbilla/bi8s-go/test

go 1.25
