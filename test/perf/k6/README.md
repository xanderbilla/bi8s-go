# k6 performance scenarios

Lightweight load profiles for the bi8s API. Run after `make docker-up`:

```bash
# read-heavy steady-state browse
BASE_URL=http://localhost:8080 k6 run test/perf/k6/read_heavy.js

# write-heavy admin validation surface
BASE_URL=http://localhost:8080 k6 run test/perf/k6/write_heavy.js
```

Thresholds enforced (script will fail-fast on regression):

| Profile       | p95 latency | error budget          |
| ------------- | ----------- | --------------------- |
| `read_heavy`  | < 500 ms    | < 1 % http failures   |
| `write_heavy` | < 800 ms    | < 1 % envelope misses |

`write_heavy` posts empty multipart bodies on purpose — the contract under
test is the structured 4xx envelope, **not** a successful write. Do **not**
point this at production.
