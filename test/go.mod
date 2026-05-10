// Module for out-of-tree integration and e2e tests. Production code lives in
// the `app/` module; this module exists so tests can be vendored / built
// independently and so `go test ./test/...` does not pollute `app`'s module
// graph.
module github.com/xanderbilla/bi8s-go/test

go 1.25

require github.com/getkin/kin-openapi v0.132.0

require (
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
