# testdata/

Per-test binary inputs (sample HLS segments, thumbnails, image blobs).

Go's `go test` toolchain treats `testdata/` directories specially — they are
ignored by build/lint and may be referenced by tests via relative paths.

Keep this folder small. Large media should be downloaded on demand from S3
and cached, not committed to git.
