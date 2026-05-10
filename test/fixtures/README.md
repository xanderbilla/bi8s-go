# fixtures/

Shared, hand-curated request/response payloads and seed data used by tests in
sibling folders.

- Group fixtures by domain: `movie/`, `person/`, `attribute/`, `encoder/`.
- Use `.json` for HTTP payloads and `.golden` for expected outputs.
- Update goldens with `go test -update ./...` (when supported by the test).
- Do **not** put binary media here — use `../testdata/` instead.
